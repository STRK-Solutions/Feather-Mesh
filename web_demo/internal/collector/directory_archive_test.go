package collector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenDirectoryArchivePrivateSameFilesystemAndReadback(t *testing.T) {
	base := t.TempDir()
	archiveDir := filepath.Join(base, "archive")
	if err := os.Mkdir(archiveDir, 0700); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(base, "events.db")
	if err := os.WriteFile(database, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := OpenDirectoryArchive(archiveDir, database)
	if err != nil {
		t.Fatal(err)
	}
	key := "research/events/" + strings.Repeat("a", 36) + "/" + strings.Repeat("b", 64) + ".json"
	ctx := context.Background()
	if err := verifiedPut(ctx, a, key, []byte("bounded synthetic event")); err != nil {
		t.Fatal(err)
	}
	listed, err := a.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].Key != key {
		t.Fatalf("private archive listing: %v %v", listed, err)
	}
	if err := verifiedDelete(ctx, a, key); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(archiveDir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDirectoryArchive(archiveDir, database); err == nil {
		t.Fatal("shared archive directory accepted")
	}
}
