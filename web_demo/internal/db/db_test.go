package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitLifecycleAndMigrationDrift(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.db")
	m := []Migration{{1, `CREATE TABLE items(id TEXT PRIMARY KEY)`}}
	if _, e := Open(p, m); e == nil {
		t.Fatal("startup created absent DB")
	}
	if _, e := os.Stat(p); !os.IsNotExist(e) {
		t.Fatal("missing file created")
	}
	d, e := Initialize(p, m)
	if e != nil {
		t.Fatal(e)
	}
	d.Close()
	if _, e := Initialize(p, m); e == nil {
		t.Fatal("reinitialization succeeded")
	}
	newer := append(append([]Migration{}, m...), Migration{2, `ALTER TABLE items ADD COLUMN value TEXT`})
	if _, e := Open(p, newer); e == nil {
		t.Fatal("startup migrated schema")
	}
	d, e = Migrate(p, newer)
	if e != nil {
		t.Fatal(e)
	}
	d.Close()
	if _, e := Open(p, m); e == nil {
		t.Fatal("incompatible rollback accepted")
	}
	changed := append([]Migration{}, newer...)
	changed[0].SQL += ";"
	if _, e := Open(p, changed); e == nil {
		t.Fatal("changed migration accepted")
	}
	d, e = Open(p, newer)
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	var fk, sync int
	var mode string
	d.QueryRow(`PRAGMA foreign_keys`).Scan(&fk)
	d.QueryRow(`PRAGMA synchronous`).Scan(&sync)
	d.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	if fk != 1 || sync != 2 || mode != "wal" {
		t.Fatalf("unsafe pragmas %d %d %s", fk, sync, mode)
	}
}
func TestRejectSymlinkAndPublicDB(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.db")
	d, e := Initialize(p, nil)
	if e != nil {
		t.Fatal(e)
	}
	d.Close()
	link := p + ".link"
	if e = os.Symlink(p, link); e != nil {
		t.Fatal(e)
	}
	if _, e = Open(link, nil); e == nil {
		t.Fatal("symlink accepted")
	}
	os.Chmod(p, 0644)
	if _, e = Open(p, nil); e == nil {
		t.Fatal("public DB accepted")
	}
}
