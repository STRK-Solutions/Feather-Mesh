package capability

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHashScopeExpiryAndRevocation(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	path := filepath.Join(t.TempDir(), "cap.sqlite")
	s, err := Initialize(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	c := Claims{AccountID: id, WorkspaceID: id, DeploymentID: id, AuthVersion: 1, WorkspaceGeneration: 1, GrantVersion: 1, ActivationGeneration: 1, Operation: "model", ExpiresAt: time.Now().UTC().Add(time.Minute)}
	token, err := s.Mint(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Check(context.Background(), token, id, "model"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][2]string{{"00000000-0000-4000-8000-000000000002", "model"}, {id, "event"}} {
		if _, err = s.Check(context.Background(), token, args[0], args[1]); err == nil {
			t.Fatal("cross-scope accepted")
		}
	}
	for _, file := range []string{path, path + "-wal"} {
		b, _ := os.ReadFile(file)
		if strings.Contains(string(b), token) {
			t.Fatal("capability persisted raw")
		}
	}
	if err = s.Revoke(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Check(context.Background(), token, id, "model"); err == nil {
		t.Fatal("revoked accepted")
	}
	c.ExpiresAt = time.Now().Add(2 * time.Hour)
	if _, err = s.Mint(context.Background(), c); err == nil {
		t.Fatal("long-lived capability")
	}
}
