package lifecycle

import (
	"encoding/json"
	"testing"
)

func TestRecordedRuntimeMountsAndEnvironmentCannotDrift(t *testing.T) {
	s, w, _ := fixture(t)
	payload := []byte(`{"Env":["FEAM_DEMO_MODE=hosted"],"HostConfig":{"Mounts":[{"Type":"bind","Source":"/private/slot-01","Target":"/workspace"},{"Type":"bind","Source":"/releases/approved/serving","Target":"/datasets/demo/serving","ReadOnly":true}],"Tmpfs":{"/tmp":"size=64m"},"ShmSize":16777216}}`)
	if e := s.RecordTemplate(w, payload); e != nil {
		t.Fatal(e)
	}
	if e := s.RecordTemplate(w, payload); e != nil {
		t.Fatal(e)
	}
	if e := s.RecordTemplate(w, append(payload, ' ')); e == nil {
		t.Fatal("existing template overwritten")
	}
	template, e := s.RuntimeTemplate(w)
	if e != nil {
		t.Fatal(e)
	}
	observed := map[string]any{"Config": map[string]any{"Env": []string{"PATH=/usr/bin", "FEAM_DEMO_MODE=hosted"}}, "HostConfig": map[string]any{"Tmpfs": map[string]string{"/tmp": "size=64m"}, "ShmSize": 16777216}, "Mounts": []map[string]any{{"Type": "bind", "Source": "/private/slot-01", "Destination": "/workspace", "RW": true}, {"Type": "bind", "Source": "/releases/approved/serving", "Destination": "/datasets/demo/serving", "RW": false}}}
	b, _ := json.Marshal(observed)
	if e = template.verify(b); e != nil {
		t.Fatal(e)
	}
	for _, change := range []string{"path", "write", "extra", "environment", "tmpfs"} {
		t.Run(change, func(t *testing.T) {
			var copy map[string]any
			_ = json.Unmarshal(b, &copy)
			switch change {
			case "path":
				copy["Mounts"].([]any)[1].(map[string]any)["Source"] = "/private/other-user"
			case "write":
				copy["Mounts"].([]any)[1].(map[string]any)["RW"] = true
			case "extra":
				copy["Mounts"] = append(copy["Mounts"].([]any), map[string]any{"Type": "bind", "Source": "/", "Destination": "/host", "RW": false})
			case "environment":
				copy["Config"].(map[string]any)["Env"] = []string{"FEAM_DEMO_MODE=manual"}
			case "tmpfs":
				copy["HostConfig"].(map[string]any)["Tmpfs"] = map[string]string{"/tmp": "size=4g"}
			}
			bad, _ := json.Marshal(copy)
			if template.verify(bad) == nil {
				t.Fatal("runtime drift accepted")
			}
		})
	}
}
