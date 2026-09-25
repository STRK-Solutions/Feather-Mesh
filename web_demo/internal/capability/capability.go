// Package capability stores only hashes of workspace-scoped bearer capabilities.
package capability

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
)

var ErrDenied = errors.New("capability denied")

type Claims struct {
	AccountID            string    `json:"account_id"`
	WorkspaceID          string    `json:"workspace_id"`
	DeploymentID         string    `json:"deployment_id"`
	AuthVersion          int64     `json:"auth_version"`
	WorkspaceGeneration  int64     `json:"workspace_generation"`
	GrantVersion         int64     `json:"grant_version"`
	ActivationGeneration int64     `json:"activation_generation"`
	Operation            string    `json:"operation"`
	ExpiresAt            time.Time `json:"expires_at"`
}
type Store struct{ DB *sql.DB }

var migrations = []db.Migration{{Version: 1, SQL: `CREATE TABLE capabilities(hash TEXT PRIMARY KEY,workspace_id TEXT NOT NULL,claims TEXT NOT NULL,revoked INTEGER NOT NULL DEFAULT 0 CHECK(revoked IN(0,1)));`}}

func Initialize(path string) (*Store, error) {
	d, err := db.Initialize(path, migrations)
	if err != nil {
		return nil, err
	}
	return &Store{d}, nil
}
func Open(path string) (*Store, error) {
	d, err := db.Open(path, migrations)
	if err != nil {
		return nil, err
	}
	return &Store{d}, nil
}
func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
func (c Claims) Valid(now time.Time) bool {
	return budget.ValidID(c.AccountID) && budget.ValidID(c.WorkspaceID) && budget.ValidID(c.DeploymentID) && c.AuthVersion > 0 && c.WorkspaceGeneration > 0 && c.GrantVersion > 0 && c.ActivationGeneration > 0 && (c.Operation == "model" || c.Operation == "event") && now.Before(c.ExpiresAt) && c.ExpiresAt.Sub(now) <= time.Hour
}
func (s *Store) Mint(ctx context.Context, c Claims) (string, error) {
	if !c.Valid(time.Now()) {
		return "", ErrDenied
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	b, _ := json.Marshal(c)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO capabilities(hash,workspace_id,claims) VALUES(?,?,?)`, tokenHash(token), c.WorkspaceID, string(b))
	return token, err
}
func (s *Store) Check(ctx context.Context, token, workspace, operation string) (Claims, error) {
	var c Claims
	if len(token) != 43 {
		return c, ErrDenied
	}
	var b string
	if err := s.DB.QueryRowContext(ctx, `SELECT claims FROM capabilities WHERE hash=? AND workspace_id=? AND revoked=0`, tokenHash(token), workspace).Scan(&b); err != nil {
		return c, ErrDenied
	}
	if json.Unmarshal([]byte(b), &c) != nil || !c.Valid(time.Now()) || c.WorkspaceID != workspace || c.Operation != operation {
		return Claims{}, ErrDenied
	}
	return c, nil
}
func (s *Store) Revoke(ctx context.Context, workspace string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE capabilities SET revoked=1 WHERE workspace_id=?`, workspace)
	return err
}
