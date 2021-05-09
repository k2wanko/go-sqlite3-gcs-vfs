package gcsvfs

import (
	"database/sql"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

var (
	_ sqlite3.VFS = &GCSVFS{}
	_ io.ReaderAt = &GCSFile{}
	_ io.WriterAt = &GCSFile{}
)

const testBucketName = "k2wanko-sqlite3"

func TempFilename(t *testing.T) string {
	return fmt.Sprintf("%s/%s_%v.db", testBucketName, t.Name(), time.Now().Unix())
}

func TestGCSVFS(t *testing.T) {
	t.Helper()
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?vfs=gcs", TempFilename(t)))
	if err != nil {
		return
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE foo (id INTEGER NOT NULL PRIMARY KEY, name TEXT);")
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.Prepare("INSERT INTO foo(id, name) values(?, ?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for i := 0; i < 10; i++ {
		_, err = stmt.Exec(i, fmt.Sprintf("test_%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	tx.Commit()

	rows, err := db.Query("SELECT id, name FROM foo")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var (
			id   int
			name string
		)
		err = rows.Scan(&id, &name)
		if err != nil {
			t.Fatal(err)
		}
		if id != i {
			t.Errorf("id = %d, want = %d", id, i)
		}
		if name != fmt.Sprintf("test_%d", i) {
			t.Errorf("name = %s, want = test_%d", name, i)
		}
		i++
	}
}
