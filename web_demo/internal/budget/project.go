package budget

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type ProjectRun struct {
	Document SignedAllocation `json:"document"`
	State    string           `json:"state"`
	Settled  int64            `json:"settled_usd_micros"`
	Evidence string           `json:"reconciliation_sha256,omitempty"`
}
type Audit struct {
	Revision int64     `json:"revision"`
	At       time.Time `json:"at"`
	Actor    string    `json:"actor"`
	Action   string    `json:"action"`
	Target   string    `json:"target"`
}
type Project struct {
	Protocol string                `json:"protocol"`
	ID       string                `json:"project_id"`
	Revision int64                 `json:"revision"`
	Ceiling  int64                 `json:"allowance_usd_micros"`
	Settled  int64                 `json:"settled_usd_micros"`
	Runs     map[string]ProjectRun `json:"runs"`
	Audit    []Audit               `json:"audit"`
}

// ProjectStore lives ONLY on the operator machine. The 32-byte encryption key
// and Ed25519 signing key are kept separately; neither is copied to demo compute.
// Each operation locks, validates, records encrypted audit, replaces and fsyncs.
// An interrupted audit/snapshot pair fails closed; it is never initialized anew.
type ProjectStore struct {
	Path string
	Key  []byte
}

func privateFile(path string) error {
	s, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !s.Mode().IsRegular() || s.Mode().Perm()&0077 != 0 {
		return errors.New("operator file must be regular and private")
	}
	return nil
}

func (s ProjectStore) crypt() (cipher.AEAD, error) {
	if len(s.Key) != 32 {
		return nil, ErrInvalid
	}
	c, err := aes.NewCipher(s.Key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(c)
}
func (s ProjectStore) encode(b []byte) ([]byte, error) {
	a, err := s.crypt()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return a.Seal(nonce, nonce, b, []byte(Protocol)), nil
}
func (s ProjectStore) decode(b []byte) ([]byte, error) {
	a, err := s.crypt()
	if err != nil {
		return nil, err
	}
	if len(b) < a.NonceSize() {
		return nil, ErrInvalid
	}
	return a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte(Protocol))
}

func (p Project) Exposure() int64 {
	x := p.Settled
	for _, r := range p.Runs {
		if r.State != "closed" {
			x += r.Document.Allocation.Amount
		}
	}
	return x
}
func (p Project) validate() error {
	if p.Protocol != Protocol || !ValidID(p.ID) || p.Revision < 1 || p.Ceiling < 1 || p.Ceiling > 1_000_000_000_000 || p.Settled < 0 || p.Exposure() > p.Ceiling || int64(len(p.Audit)) != p.Revision {
		return ErrInvalid
	}
	for id, r := range p.Runs {
		if id != r.Document.Allocation.ID || r.Document.Allocation.ProjectID != p.ID || r.Document.Allocation.Validate() != nil || (r.State != "active" && r.State != "unknown" && r.State != "closed") || r.Settled < 0 || r.Settled > r.Document.Allocation.Amount {
			return ErrInvalid
		}
	}
	for i, a := range p.Audit {
		if a.Revision != int64(i+1) || a.Actor == "" || a.At.IsZero() {
			return ErrInvalid
		}
	}
	return nil
}

func (s ProjectStore) lock() (*os.File, error) {
	if len(s.Key) != 32 {
		return nil, ErrInvalid
	}
	parent, err := os.Stat(filepath.Dir(s.Path))
	if err != nil {
		return nil, err
	}
	if !parent.IsDir() || parent.Mode().Perm()&0077 != 0 {
		return nil, errors.New("operator directory must be private")
	}
	f, err := os.OpenFile(s.Path+".lock", os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err = privateFile(s.Path + ".lock"); err != nil {
		f.Close()
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
func unlock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }

func (s ProjectStore) load() (Project, error) {
	var p Project
	if err := privateFile(s.Path); err != nil {
		return p, err
	}
	if err := privateFile(s.Path + ".audit"); err != nil {
		return p, err
	}
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return p, err
	}
	if len(b) > 64<<20 {
		return p, ErrInvalid
	}
	plain, err := s.decode(b)
	if err != nil {
		return p, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(plain))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || p.validate() != nil {
		return Project{}, ErrInvalid
	}
	// The final audit record authenticates exactly this snapshot. A torn write
	// cannot silently roll back an allocation or a ceiling decrease.
	ab, err := os.ReadFile(s.Path + ".audit")
	if err != nil {
		return p, err
	}
	if len(ab) > 64<<20 {
		return p, ErrInvalid
	}
	lines := bytes.Split(bytes.TrimSuffix(ab, []byte("\n")), []byte("\n"))
	if int64(len(lines)) != p.Revision {
		return p, errors.New("reconciliation_required: interrupted project update")
	}
	h := sha256.Sum256(plain)
	last, err := hex.DecodeString(string(lines[len(lines)-1]))
	if err != nil {
		return p, ErrInvalid
	}
	want, err := s.decode(last)
	if err != nil || !bytes.Equal(want, h[:]) {
		return p, errors.New("reconciliation_required: project audit mismatch")
	}
	return p, nil
}
func (s ProjectStore) save(p Project) error {
	if err := p.validate(); err != nil {
		return err
	}
	plain, err := json.Marshal(p)
	if err != nil {
		return err
	}
	encrypted, err := s.encode(plain)
	if err != nil {
		return err
	}
	h := sha256.Sum256(plain)
	audit, err := s.encode(h[:])
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.Path+".audit", os.O_APPEND|os.O_CREATE|os.O_WRONLY|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, hex.EncodeToString(audit))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".project-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(encrypted); err == nil {
		err = tmp.Sync()
	}
	if ce := tmp.Close(); err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), s.Path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(s.Path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (s ProjectStore) Initialize(id, actor string, ceiling, priorCharges int64) error {
	f, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock(f)
	for _, path := range []string{s.Path, s.Path + ".audit"} {
		if _, err = os.Lstat(path); !os.IsNotExist(err) {
			return errors.New("refusing to initialize existing or inaccessible project state")
		}
	}
	p := Project{Protocol: Protocol, ID: id, Revision: 1, Ceiling: ceiling, Settled: priorCharges, Runs: map[string]ProjectRun{}, Audit: []Audit{{1, time.Now().UTC(), actor, "initialize", id}}}
	return s.save(p)
}
func (s ProjectStore) Read() (Project, error) {
	f, err := s.lock()
	if err != nil {
		return Project{}, err
	}
	defer unlock(f)
	return s.load()
}
func (s ProjectStore) update(actor, action, target string, change func(*Project) error) (Project, error) {
	f, err := s.lock()
	if err != nil {
		return Project{}, err
	}
	defer unlock(f)
	p, err := s.load()
	if err != nil {
		return p, err
	}
	if actor == "" || len(actor) > 128 {
		return p, ErrInvalid
	}
	if err = change(&p); err != nil {
		return p, err
	}
	p.Revision++
	p.Audit = append(p.Audit, Audit{p.Revision, time.Now().UTC(), actor, action, target})
	return p, s.save(p)
}
func (s ProjectStore) Allocate(actor string, a Allocation, key ed25519.PrivateKey) (SignedAllocation, error) {
	var result SignedAllocation
	_, err := s.update(actor, "allocate", a.ID, func(p *Project) error {
		if a.ProjectID != p.ID {
			return ErrInvalid
		}
		if _, exists := p.Runs[a.ID]; exists {
			return ErrConflict
		}
		if a.Amount < 1 || a.Amount > p.Ceiling-p.Exposure() {
			return ErrExhausted
		}
		a.LedgerRevision = p.Revision + 1
		var err error
		result, err = Sign(a, key)
		if err != nil {
			return err
		}
		p.Runs[a.ID] = ProjectRun{Document: result, State: "active"}
		return nil
	})
	return result, err
}

// Reconcile requires an operator-reviewed immutable receipt hash. Unknown usage
// is represented by MarkUnknown, retaining the entire allocation indefinitely.
func (s ProjectStore) Reconcile(actor, id, evidence string, actual int64) error {
	_, err := s.update(actor, "reconcile", id, func(p *Project) error {
		r, ok := p.Runs[id]
		if !ok || actual < 0 || actual > r.Document.Allocation.Amount {
			return ErrInvalid
		}
		e, err := hex.DecodeString(evidence)
		if err != nil || len(e) != 32 {
			return ErrInvalid
		}
		if r.State == "closed" {
			if r.Settled == actual && r.Evidence == evidence {
				return nil
			}
			return ErrConflict
		}
		r.State = "closed"
		r.Settled = actual
		r.Evidence = evidence
		p.Settled += actual
		p.Runs[id] = r
		return nil
	})
	return err
}
func (s ProjectStore) MarkUnknown(actor, id string) error {
	_, err := s.update(actor, "unknown", id, func(p *Project) error {
		r, ok := p.Runs[id]
		if !ok || r.State == "closed" {
			return ErrConflict
		}
		r.State = "unknown"
		p.Runs[id] = r
		return nil
	})
	return err
}
func (s ProjectStore) SetCeiling(actor string, ceiling int64) error {
	_, err := s.update(actor, "ceiling", s.Path, func(p *Project) error {
		if ceiling < p.Exposure() {
			return ErrExhausted
		}
		p.Ceiling = ceiling
		return nil
	})
	return err
}

// ReadPrivateKey reads exactly the expected bytes without printing file content.
func ReadPrivateKey(path string, size int) ([]byte, error) {
	if err := privateFile(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(size+1)))
	if err != nil || len(b) != size {
		return nil, ErrInvalid
	}
	return b, nil
}

// InitializeKeys creates independent encryption/signing keys outside compute.
// Refusing existing paths prevents a repeated setup from orphaning old spending.
func InitializeKeys(encryptionPath, signingPath, publicPath string) error {
	paths := []string{encryptionPath, signingPath, publicPath}
	seen := map[string]bool{}
	for _, path := range paths {
		if !filepath.IsAbs(path) || seen[path] {
			return ErrInvalid
		}
		seen[path] = true
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return ErrConflict
		}
		parent, err := os.Lstat(filepath.Dir(path))
		if err != nil {
			return err
		}
		if !parent.IsDir() || parent.Mode().Perm()&0077 != 0 {
			return ErrInvalid
		}
	}
	encryption := make([]byte, 32)
	if _, err := rand.Read(encryption); err != nil {
		return err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	for i, b := range [][]byte{encryption, private, public} {
		f, err := os.OpenFile(paths[i], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		ce := f.Close()
		if err != nil {
			return err
		}
		if ce != nil {
			return ce
		}
		dir, err := os.Open(filepath.Dir(paths[i]))
		if err != nil {
			return err
		}
		err = dir.Sync()
		dir.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
