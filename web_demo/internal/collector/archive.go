package collector

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var ErrNotFound = errors.New("archive object absent")

type Archive interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

type ArchivedObject struct {
	Key   string
	Bytes int64
}
type ArchiveLister interface {
	List(context.Context) ([]ArchivedObject, error)
}

var objectRE = regexp.MustCompile(`^research/(events|exports|deletions)/[0-9a-f-]{36}/[0-9a-f]{64}\.json$`)

type R2 struct {
	AccountID, Bucket, AccessKeyID, SecretAccessKey string
	Client                                          *http.Client
}

func (a R2) call(ctx context.Context, method, key string, body []byte) ([]byte, error) {
	if !objectRE.MatchString(key) {
		return nil, ErrRejected
	}
	return a.request(ctx, method, key, "", body)
}
func (a R2) request(ctx context.Context, method, key, query string, body []byte) ([]byte, error) {
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(a.AccountID) || !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`).MatchString(a.Bucket) || a.AccessKeyID == "" || a.SecretAccessKey == "" || len(body) > MaxBatch {
		return nil, ErrRejected
	}
	target := "https://" + a.AccountID + ".r2.cloudflarestorage.com/" + a.Bucket
	if key != "" {
		target += "/" + key
	}
	if query != "" {
		target += "?" + query
	}
	req, e := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-amz-content-sha256", digest(body))
	if e = v4.NewSigner().SignHTTP(ctx, aws.Credentials{AccessKeyID: a.AccessKeyID, SecretAccessKey: a.SecretAccessKey}, req, digest(body), "s3", "auto", time.Now()); e != nil {
		return nil, errors.New("archive signing failed")
	}
	client := a.Client
	if client == nil {
		client = &http.Client{}
	}
	safe := *client
	safe.Timeout = 10 * time.Second
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, e := safe.Do(req)
	if e != nil {
		return nil, errors.New("archive unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errors.New("archive request rejected")
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, MaxBatch+1))
	if e != nil || len(b) > MaxBatch {
		return nil, errors.New("archive response exceeds bound")
	}
	return b, nil
}

func (a R2) List(ctx context.Context) ([]ArchivedObject, error) {
	var out []ArchivedObject
	token := ""
	seen := map[string]bool{}
	for page := 0; page < 9; page++ {
		q := url.Values{"list-type": {"2"}, "prefix": {"research/"}, "max-keys": {"1000"}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		b, e := a.request(ctx, "GET", "", q.Encode(), nil)
		if e != nil {
			return nil, e
		}
		var response struct {
			XMLName   xml.Name `xml:"ListBucketResult"`
			Name      string   `xml:"Name"`
			Prefix    string   `xml:"Prefix"`
			Truncated bool     `xml:"IsTruncated"`
			Next      string   `xml:"NextContinuationToken"`
			Contents  []struct {
				Key  string `xml:"Key"`
				Size int64  `xml:"Size"`
			} `xml:"Contents"`
		}
		if xml.Unmarshal(b, &response) != nil || response.Name != a.Bucket || response.Prefix != "research/" || len(response.Contents) > 1000 {
			return nil, ErrRejected
		}
		for _, v := range response.Contents {
			if !objectRE.MatchString(v.Key) || v.Size < 0 || v.Size > MaxBatch {
				return nil, errors.New("unknown archive object in research prefix")
			}
			out = append(out, ArchivedObject{v.Key, v.Size})
		}
		if len(out) > 8000 {
			return nil, errors.New("archive listing bound exceeded")
		}
		if !response.Truncated {
			return out, nil
		}
		if response.Next == "" || len(response.Next) > 4096 || seen[response.Next] {
			return nil, errors.New("invalid archive continuation")
		}
		seen[response.Next] = true
		token = response.Next
	}
	return nil, errors.New("archive listing page bound exceeded")
}
func (a R2) Put(ctx context.Context, k string, b []byte) error {
	_, e := a.call(ctx, "PUT", k, b)
	return e
}
func (a R2) Get(ctx context.Context, k string) ([]byte, error) { return a.call(ctx, "GET", k, nil) }
func (a R2) Delete(ctx context.Context, k string) error {
	_, e := a.call(ctx, "DELETE", k, nil)
	if errors.Is(e, ErrNotFound) {
		return nil
	}
	return e
}

// DirectoryArchive keeps verified objects on the same private filesystem as the
// collector database. A verified copy is required before destructive teardown.
type DirectoryArchive struct{ Root string }

// OpenDirectoryArchive refuses a missing, shared or foreign filesystem path.
// The production entrypoint also pins Root to the reviewed Ubuntu directory.
func OpenDirectoryArchive(root, database string) (DirectoryArchive, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || !filepath.IsAbs(database) || filepath.Clean(database) != database {
		return DirectoryArchive{}, ErrRejected
	}
	dir, e := os.Lstat(root)
	if e != nil || !dir.IsDir() || dir.Mode().Perm() != 0700 {
		return DirectoryArchive{}, ErrRejected
	}
	db, e := os.Lstat(database)
	if e != nil || !db.Mode().IsRegular() || db.Mode().Perm()&0077 != 0 {
		return DirectoryArchive{}, ErrRejected
	}
	dirStat, dirOK := dir.Sys().(*syscall.Stat_t)
	dbStat, dbOK := db.Sys().(*syscall.Stat_t)
	if !dirOK || !dbOK || dirStat.Uid != uint32(os.Geteuid()) || dbStat.Uid != uint32(os.Geteuid()) || dirStat.Dev != dbStat.Dev || dbStat.Nlink != 1 {
		return DirectoryArchive{}, ErrRejected
	}
	return DirectoryArchive{Root: root}, nil
}

func (a DirectoryArchive) List(_ context.Context) ([]ArchivedObject, error) {
	fi, e := os.Lstat(a.Root)
	if e != nil || !fi.IsDir() || fi.Mode().Perm() != 0700 {
		return nil, ErrRejected
	}
	entries, e := os.ReadDir(a.Root)
	if e != nil || len(entries) > 8000 {
		return nil, ErrRejected
	}
	var out []ArchivedObject
	for _, entry := range entries {
		key := strings.ReplaceAll(entry.Name(), "_", "/")
		info, e := entry.Info()
		if e != nil {
			return nil, e
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || !objectRE.MatchString(key) || info.Size() > MaxBatch {
			return nil, errors.New("unknown local archive object")
		}
		out = append(out, ArchivedObject{key, info.Size()})
	}
	return out, nil
}

func (a DirectoryArchive) path(k string) (string, error) {
	if !objectRE.MatchString(k) {
		return "", ErrRejected
	}
	fi, e := os.Lstat(a.Root)
	if e != nil || !fi.IsDir() || fi.Mode().Perm() != 0700 {
		return "", ErrRejected
	}
	return filepath.Join(a.Root, strings.ReplaceAll(k, "/", "_")), nil
}
func (a DirectoryArchive) Put(_ context.Context, k string, b []byte) error {
	p, e := a.path(k)
	if e != nil || len(b) > MaxBatch {
		return ErrRejected
	}
	f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(e) {
		old, e := a.Get(context.Background(), k)
		if e == nil && digest(old) == digest(b) {
			return nil
		}
		return errors.New("archive object conflict")
	}
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	dir, e := os.Open(a.Root)
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}
func (a DirectoryArchive) Get(_ context.Context, k string) ([]byte, error) {
	p, e := a.path(k)
	if e != nil {
		return nil, e
	}
	fi, e := os.Lstat(p)
	if os.IsNotExist(e) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || fi.Size() > MaxBatch {
		return nil, ErrRejected
	}
	f, e := os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(fi, opened) {
		return nil, ErrRejected
	}
	b, e := io.ReadAll(io.LimitReader(f, MaxBatch+1))
	if e != nil || len(b) > MaxBatch {
		return nil, ErrRejected
	}
	return b, nil
}
func (a DirectoryArchive) Delete(_ context.Context, k string) error {
	p, e := a.path(k)
	if e != nil {
		return e
	}
	e = os.Remove(p)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	dir, e := os.Open(a.Root)
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}
func verifiedPut(ctx context.Context, a Archive, key string, b []byte) error {
	if e := a.Put(ctx, key, b); e != nil {
		return e
	}
	got, e := a.Get(ctx, key)
	if e != nil || digest(got) != digest(b) {
		return errors.New("archive readback verification failed")
	}
	return nil
}
func verifiedDelete(ctx context.Context, a Archive, key string) error {
	if e := a.Delete(ctx, key); e != nil {
		return e
	}
	if _, e := a.Get(ctx, key); !errors.Is(e, ErrNotFound) {
		return errors.New("archive deletion not verified")
	}
	return nil
}

// Flush writes immutable participant batches. A durable pending record precedes
// upload, so crash retries use the exact same object and payload hash.
func (s *Store) Flush(ctx context.Context, a Archive) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.makeBatches(ctx); e != nil {
		return e
	}
	for _, table := range []string{"batches", "exports"} {
		rows, e := s.DB.QueryContext(ctx, `SELECT id,object_key,body,sha256 FROM `+table+` WHERE status='pending' ORDER BY id`)
		if e != nil {
			return e
		}
		type batch struct{ id, key, body, hash string }
		var pending []batch
		for rows.Next() {
			var b batch
			if e = rows.Scan(&b.id, &b.key, &b.body, &b.hash); e != nil {
				rows.Close()
				return e
			}
			pending = append(pending, b)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, b := range pending {
			if digest([]byte(b.body)) != b.hash {
				return errors.New("batch integrity failure")
			}
			if e = verifiedPut(ctx, a, b.key, []byte(b.body)); e != nil {
				return e
			}
			if table == "batches" {
				_, e = s.DB.ExecContext(ctx, `UPDATE batches SET status='verified',verified_at=? WHERE id=? AND status='pending'`, s.Now().UTC().Format(time.RFC3339Nano), b.id)
			} else {
				_, e = s.DB.ExecContext(ctx, `UPDATE exports SET status='verified' WHERE id=? AND status='pending'`, b.id)
			}
			if e != nil {
				return e
			}
		}
	}

	return nil
}
func (s *Store) makeBatches(ctx context.Context) error {
	for {
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		var participant string
		e = tx.QueryRow(`SELECT participant FROM events WHERE batch_id IS NULL AND participant NOT IN (SELECT participant FROM deletions) ORDER BY position LIMIT 1`).Scan(&participant)
		if errors.Is(e, sql.ErrNoRows) {
			tx.Rollback()
			return nil
		}
		if e != nil {
			tx.Rollback()
			return e
		}
		rows, e := tx.Query(`SELECT id,body,received_at FROM events WHERE participant=? AND batch_id IS NULL ORDER BY position LIMIT 512`, participant)
		if e != nil {
			tx.Rollback()
			return e
		}
		var ids []string
		var records []json.RawMessage
		var latest time.Time
		size := 2
		for rows.Next() {
			var id, body, received string
			if e = rows.Scan(&id, &body, &received); e != nil {
				break
			}
			if size+len(body)+1 > MaxBatch {
				break
			}
			ids = append(ids, id)
			stamp, parseErr := time.Parse(time.RFC3339Nano, received)
			if parseErr != nil {
				e = parseErr
				break
			}
			if stamp.After(latest) {
				latest = stamp
			}
			records = append(records, json.RawMessage(body))
			size += len(body) + 1
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			tx.Rollback()
			return e
		}
		body, _ := json.Marshal(records)
		hash := digest(body)
		var archiveBytes int64
		if e = tx.QueryRow(archiveUsageSQL).Scan(&archiveBytes); e != nil || s.MaxArchiveBytes <= 0 || archiveBytes+int64(len(body)) > s.MaxArchiveBytes {
			tx.Rollback()
			return ErrBackpressure
		}
		id := control.ID()
		key := "research/events/" + participant + "/" + hash + ".json"
		if _, e = tx.Exec(`INSERT INTO batches(id,participant,object_key,body,sha256,status,verified_at,archive_bytes,expires_at) VALUES(?,?,?,?,?,'pending',NULL,?,?)`, id, participant, key, string(body), hash, len(body), latest.Add(30*24*time.Hour).UTC().Format(time.RFC3339Nano)); e != nil {
			tx.Rollback()
			return e
		}
		for _, eventID := range ids {
			if _, e = tx.Exec(`UPDATE events SET batch_id=? WHERE id=?`, id, eventID); e != nil {
				tx.Rollback()
				return e
			}
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
}
