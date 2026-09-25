package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
)

type runtimeTemplate struct {
	Env        []string `json:"Env"`
	HostConfig struct {
		Mounts []struct {
			Type, Source, Target string
			ReadOnly             bool
		} `json:"Mounts"`
		Tmpfs   map[string]string `json:"Tmpfs"`
		ShmSize int64             `json:"ShmSize"`
	} `json:"HostConfig"`
}

func (s *Store) RecordTemplate(w Workspace, payload []byte) error {
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	_, e := s.DB.Exec(`INSERT OR IGNORE INTO runtime_templates VALUES(?,?,?,?)`, w.ID, w.Generation, string(payload), digest)
	if e != nil {
		return e
	}
	var recorded string
	if e = s.DB.QueryRow(`SELECT sha256 FROM runtime_templates WHERE workspace_id=? AND generation=?`, w.ID, w.Generation).Scan(&recorded); e != nil {
		return e
	}
	if recorded != digest {
		return errors.New("generation runtime template changed")
	}
	return nil
}
func (s *Store) RuntimeTemplate(w Workspace) (runtimeTemplate, error) {
	var raw, digest string
	var out runtimeTemplate
	e := s.DB.QueryRow(`SELECT payload,sha256 FROM runtime_templates WHERE workspace_id=? AND generation=?`, w.ID, w.Generation).Scan(&raw, &digest)
	if e != nil {
		return out, errors.New("recorded generation template missing")
	}
	sum := sha256.Sum256([]byte(raw))
	if hex.EncodeToString(sum[:]) != digest {
		return out, errors.New("runtime template digest mismatch")
	}
	e = json.Unmarshal([]byte(raw), &out)
	return out, e
}
func (t runtimeTemplate) verify(raw []byte) error {
	var observed struct {
		Config     struct{ Env []string }
		HostConfig struct {
			Tmpfs   map[string]string
			ShmSize int64
		}
		Mounts []struct {
			Type, Source, Destination string
			RW                        bool
		}
	}
	if json.Unmarshal(raw, &observed) != nil {
		return errors.New("invalid runtime inspection")
	}
	if !reflect.DeepEqual(t.HostConfig.Tmpfs, observed.HostConfig.Tmpfs) || t.HostConfig.ShmSize != observed.HostConfig.ShmSize {
		return errors.New("runtime tmpfs drift")
	}
	if len(observed.Mounts) != len(t.HostConfig.Mounts) {
		return errors.New("runtime mount inventory drift")
	}
	seen := map[string]bool{}
	for _, actual := range observed.Mounts {
		if seen[actual.Destination] {
			return errors.New("duplicate runtime mount")
		}
		seen[actual.Destination] = true
		matched := false
		for _, expected := range t.HostConfig.Mounts {
			if actual.Destination == expected.Target && actual.Source == expected.Source && actual.Type == expected.Type && actual.RW != expected.ReadOnly {
				matched = true
				break
			}
		}
		if !matched {
			return errors.New("runtime mount authority drift")
		}
	}
	// Runtime-selected variables must occur exactly once with the fixed value;
	// additional image-baked environment is not an authority input.
	for _, required := range t.Env {
		key := required
		for i, c := range required {
			if c == '=' {
				key = required[:i+1]
				break
			}
		}
		count := 0
		for _, actual := range observed.Config.Env {
			if len(actual) >= len(key) && actual[:len(key)] == key {
				count++
				if actual != required {
					return errors.New("runtime environment drift")
				}
			}
		}
		if count != 1 {
			return errors.New("runtime environment missing or duplicate")
		}
	}
	return nil
}
