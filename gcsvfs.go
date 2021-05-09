package gcsvfs

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/mattn/go-sqlite3"
	"google.golang.org/api/googleapi"
)

// GCSVFS implements sqlite.VFS.
type GCSVFS struct {
	ctx    context.Context
	client *storage.Client

	pathInfos map[string]*pathInfo
}

// GCSFile implements vfs file.
type GCSFile struct {
	ctx context.Context
	vfs *GCSVFS

	pendingWrite bool
	tempFile     *os.File

	bucket string
	obj    *storage.ObjectHandle
	rc     io.ReaderAt

	mu sync.Mutex
}

type pathInfo struct {
	bucket string
	path   string
}

func parsePathInfo(path string) (info *pathInfo, err error) {
	part := strings.Split(path, "/")
	if len(part) < 2 {
		return nil, errors.New("gcsvfs: parse path error. require [bucket]/[object_path]")
	}
	return &pathInfo{
		bucket: part[0],
		path:   strings.Join(part[1:], "/"),
	}, nil
}

func (vfs *GCSVFS) FullPathname(path string) (fullpath string, err error) {
	info, err := parsePathInfo(path)
	if err != nil {
		return
	}

	fullpath = filepath.Join(os.TempDir(), info.path)

	if vfs.pathInfos == nil {
		vfs.pathInfos = make(map[string]*pathInfo)
	}

	vfs.pathInfos[fullpath] = info
	return
}

func (vfs *GCSVFS) Delete(path string, dirSync int) error {
	info := vfs.lookupPathInfo(path)
	obj := vfs.client.Bucket(info.bucket).Object(info.path)
	return obj.Delete(vfs.ctx)
}

func (vfs *GCSVFS) Open(name string, flags int) (file interface{}, err error) {
	if vfs.ctx == nil {
		vfs.ctx = context.Background()
	}
	ctx := context.Background()
	if vfs.client == nil {
		client, err := storage.NewClient(ctx)
		if err != nil {
			return nil, err
		}
		vfs.client = client
	}

	pathInfo := vfs.lookupPathInfo(name)
	obj := vfs.client.Bucket(pathInfo.bucket).Object(pathInfo.path)
	tempFile, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		return nil, err
	}

	file = &GCSFile{
		ctx:      ctx,
		vfs:      vfs,
		tempFile: tempFile,
		obj:      obj,
	}
	return
}

func (vfs *GCSVFS) lookupPathInfo(path string) *pathInfo {
	infoKey := path
	suffix := ""
	if strings.HasSuffix(infoKey, "-journal") {
		infoKey = strings.TrimSuffix(infoKey, "-journal")
		suffix = "-journal"
	}
	if strings.HasSuffix(infoKey, "-wal") {
		infoKey = strings.TrimSuffix(infoKey, "-wal")
		suffix = "-wal"
	}

	info := vfs.pathInfos[infoKey]
	return &pathInfo{
		bucket: info.bucket,
		path:   info.path + suffix,
	}
}

func (f *GCSFile) Close() (err error) {
	if f.tempFile != nil {
		f.tempFile.Close()
		os.Remove(f.tempFile.Name())
		delete(f.vfs.pathInfos, f.tempFile.Name())
	}
	return
}

func (f *GCSFile) FileSize() (size int64, err error) {
	attrs, err := f.obj.Attrs(f.ctx)
	if err == storage.ErrObjectNotExist {
		return 0, nil
	}
	if err != nil {
		return
	}
	size = attrs.Size
	return
}

func (f *GCSFile) WriteAt(b []byte, off int64) (n int, err error) {
	f.pendingWrite = true
	return f.tempFile.WriteAt(b, off)
}

func (f *GCSFile) Sync(flags int) (err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pendingWrite {
		w := f.obj.NewWriter(f.ctx)
		defer w.Close()
		f.tempFile.Seek(0, 0)
		if _, err := io.Copy(w, f.tempFile); err != nil {
			return err
		}
		f.pendingWrite = false
	}
	return
}

func (f *GCSFile) ReadAt(b []byte, off int64) (n int, err error) {
	var r *storage.Reader
	r, err = f.obj.NewRangeReader(f.ctx, off, int64(len(b)))
	if err == storage.ErrObjectNotExist {
		return 0, nil
	} else if e, ok := err.(*googleapi.Error); ok {
		switch e.Code {
		case http.StatusRequestedRangeNotSatisfiable:
			return 0, nil
		}
		return
	} else if err != nil {
		return
	}
	defer r.Close()

	n, err = r.Read(b)

	return
}

func init() {
	sqlite3.VFSRegister("gcs", &GCSVFS{})
}
