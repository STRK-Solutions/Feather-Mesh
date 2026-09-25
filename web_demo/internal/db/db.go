// Package db owns explicit, hash-checked SQLite schema lifecycle. Callers must
// provision and verify a local filesystem before opening the owner-only file.
package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Migration struct {
	Version int
	SQL     string
}

func connect(path string) (*sql.DB, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("database requires absolute path")
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 {
		return nil, errors.New("database must be an owner-only regular file")
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("mode", "rw")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Add("_pragma", "busy_timeout(5000)")
	u.RawQuery = q.Encode()
	d, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)
	if err = d.Ping(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}
func Initialize(path string, migrations []Migration) (*sql.DB, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	d, err := connect(path)
	if err != nil {
		return nil, err
	}
	if _, err = d.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,sha256 TEXT NOT NULL,applied_at TEXT NOT NULL)`); err == nil {
		err = apply(d, migrations)
	}
	if err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}
func Open(path string, migrations []Migration) (*sql.DB, error) {
	d, err := connect(path)
	if err != nil {
		return nil, err
	}
	if err = verify(d, migrations, true); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}
func Migrate(path string, migrations []Migration) (*sql.DB, error) {
	d, err := connect(path)
	if err != nil {
		return nil, err
	}
	if err = verify(d, migrations, false); err == nil {
		err = apply(d, migrations)
	}
	if err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}
func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func verify(d *sql.DB, ms []Migration, complete bool) error {
	rows, err := d.Query(`SELECT version,sha256 FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("explicit database initialization required: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var v int
		var h string
		if err = rows.Scan(&v, &h); err != nil {
			return err
		}
		if n >= len(ms) || ms[n].Version != v || hash(ms[n].SQL) != h {
			return errors.New("unknown or changed migration; incompatible binary")
		}
		n++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if complete && n != len(ms) {
		return errors.New("explicit database migration required")
	}
	return nil
}
func apply(d *sql.DB, ms []Migration) error {
	for i, m := range ms {
		if m.Version != i+1 || m.SQL == "" {
			return errors.New("migrations must have consecutive positive versions")
		}
	}
	if err := verify(d, ms, false); err != nil {
		return err
	}
	tx, err := d.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err = tx.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		return err
	}
	for _, m := range ms[n:] {
		if _, err = tx.Exec(m.SQL); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations VALUES(?,?,?)`, m.Version, hash(m.SQL), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
