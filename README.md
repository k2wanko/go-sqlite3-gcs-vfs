# go-sqlite3-gcs-vfs

Provides a virtual file system with a [GCS](https://cloud.google.com/storage) backend for [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3).

!!! IMPORTANT !!!

This VFS assumes that you have forked go-sqlite3.
https://github.com/k2wanko/go-sqlite3/tree/vfs

### Development

```bash
mkdir -p $GOPATH/src/github.com/mattn
cd $GOPATH/src/github.com/mattn
git clone https://github.com/k2wanko/go-sqlite3
cd go-sqlite3
git switch -b vfs origin/vfs

mkdir -p $GOPATH/src/github.com/k2wanko
cd $GOPATH/src/github.com/k2wanko
git clone https://github.com/k2wanko/go-sqlite3-gcs-vfs
cd go-sqlite3-gcs-vfs

export GO111MODULE=off
go test -tags=vfs
```

#### Setting VS Code

```json:settings.json
{
    "go.testEnvVars": {
        "GO111MODULE": "off",
        "GOFLAGS": "-v -tags=vfs",
    },
    "gopls": {
        "build.env": {
            "GO111MODULE": "off",
            "GOFLAGS": "-tags=vfs",
        },
    },
}
```