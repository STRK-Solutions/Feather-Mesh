package pipeline

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// sharingCommands applies the operator-pinned web-demo UID map, never job data.
// It exposes only serving bytes. The mounted project's .feam config, provenance,
// staging and raw sources have no named reader ACL. Existing ACLs on a release
// are removed before exact grants, so retries cannot accumulate extra readers.
func (m *Manager) sharingCommands(bundle, hash string) ([][]string, error) {
	if !slug.MatchString(bundle) || !digestPattern.MatchString(hash) {
		return nil, errors.New("invalid release identity")
	}
	root := m.release(bundle, hash)
	serving := filepath.Join(root, "provider", "serving")
	return [][]string{
		{"--recursive", "--remove-all", "--", root},
		{"--modify", "u:2101:--x,u:2103:--x,u:200999:--x", "--", m.Config.Root, filepath.Join(m.Config.Root, "releases"), filepath.Join(m.Config.Root, "releases", bundle), root, filepath.Join(root, "provider"), serving},
		{"--recursive", "--modify", "u:200999:r-X", "--", serving},
	}, nil
}

func (m *Manager) shareServing(bundle, hash string) error {
	if runtime.GOOS != "linux" {
		return errors.New("web-demo ACL publication requires Linux")
	}
	commands, err := m.sharingCommands(bundle, hash)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, args := range commands {
		cmd := exec.CommandContext(ctx, "/usr/bin/setfacl", args...)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
		if err = cmd.Run(); err != nil {
			return errors.New("release serving ACL outcome requires reconciliation")
		}
	}
	return nil
}
