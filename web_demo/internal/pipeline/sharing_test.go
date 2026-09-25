package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseSharingPlanAndPrivateSeal(t *testing.T) {
	m := managerFixture(t)
	commands, e := m.sharingCommands("demo-climate", strings.Repeat("a", 64))
	if e != nil || len(commands) != 3 {
		t.Fatal(e)
	}
	if strings.Contains(strings.Join(commands[1], " "), "staging") || strings.Contains(strings.Join(commands[2], " "), "provenance") {
		t.Fatal("private bytes shared")
	}
	if commands[2][len(commands[2])-1] != filepath.Join(m.release("demo-climate", strings.Repeat("a", 64)), "provider", "serving") {
		t.Fatal("non-serving recursive grant")
	}
	for _, identity := range []string{"../escape", "/absolute", "UPPER"} {
		if _, e = m.sharingCommands(identity, strings.Repeat("a", 64)); e == nil {
			t.Fatal("unsafe identity shared")
		}
	}
	root := t.TempDir()
	private := filepath.Join(root, "provenance")
	os.Mkdir(private, 0700)
	file := filepath.Join(private, "source.json")
	os.WriteFile(file, []byte("private"), 0600)
	t.Cleanup(func() { os.Chmod(private, 0700) })
	if e = sealTree(root); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{private, file} {
		info, e := os.Stat(p)
		if e != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatal("sealed private content readable by other identities", p, e)
		}
	}
}
