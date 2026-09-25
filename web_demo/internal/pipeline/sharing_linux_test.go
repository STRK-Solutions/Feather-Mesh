//go:build linux && pipeline_integration

package pipeline

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxPromotedServingACLKeepsProvenancePrivate(t *testing.T) {
	m := realManager(t)
	m.Config.WebDemoACL = true
	j := fixtureJob()
	input := realInput(t, &j)
	s, e := m.Prepare(context.Background(), j, j.Hash(), input)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Approve(j.ID, s.CandidateHash, "00000000-0000-4000-8000-000000000099"); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Promote(j.ID); e != nil {
		t.Fatal(e)
	}
	root := m.release(j.Bundle, s.CandidateHash)
	for _, tc := range []struct {
		path   string
		reader bool
	}{
		{filepath.Join(root, "provider", "serving", "manifest.json"), true},
		{filepath.Join(root, "provider", "serving", "datasets", "daily", "v1", "observations.parquet"), true},
		{filepath.Join(root, "provider", ".feam", "project.toml"), false},
		{filepath.Join(root, "provenance"), false},
		{m.stage(j.ID), false},
	} {
		b, e := exec.Command("/usr/bin/getfacl", "--absolute-names", "--numeric", tc.path).Output()
		if e != nil {
			t.Fatal(e)
		}
		acl := string(b)
		if strings.Contains(acl, "user:200999:r") != tc.reader {
			t.Fatalf("incorrect mapped reader ACL: %s", acl)
		}
		if !strings.Contains(acl, "other::---") {
			t.Fatalf("other identity readable: %s", acl)
		}
	}
}
