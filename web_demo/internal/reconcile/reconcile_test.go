package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"io"
	"net/http"
	"strings"
	"testing"
)

type source struct {
	policy   control.Policy
	success  bool
	revision int64
}

func (s *source) Policy(context.Context) (control.Policy, error) { return s.policy, nil }
func (s *source) Result(_ context.Context, r int64, ok bool) error {
	s.success = ok
	s.revision = r
	return nil
}

type provider struct {
	groups map[string][]string
	fail   bool
}

func (p *provider) Replace(_ context.Context, id string, emails []string) error {
	if p.fail {
		return errors.New("denied")
	}
	p.groups[id] = append([]string{}, emails...)
	return nil
}
func TestExactReconciliationRemovalEmptyAndFailure(t *testing.T) {
	ctx := context.Background()
	s := &source{policy: control.Policy{Revision: 1, Users: []string{"u+tag@example.invalid", "u@example.invalid"}, Admins: []string{"a@example.invalid"}}}
	p := &provider{groups: map[string][]string{}}
	r := Reconciler{Source: s, Provider: p, UserGroup: "users", AdminGroup: "admins"}
	if e := r.Once(ctx); e != nil {
		t.Fatal(e)
	}
	if !s.success || len(p.groups["users"]) != 2 {
		t.Fatal("exact members lost")
	}
	s.policy.Revision++
	s.policy.Users = nil
	if e := r.Once(ctx); e != nil {
		t.Fatal(e)
	}
	if len(p.groups["users"]) != 0 {
		t.Fatal("removed membership restored")
	}
	if e := r.Once(ctx); e != nil || len(p.groups["users"]) != 0 {
		t.Fatal("repeat reconcile restored members")
	}
	p.fail = true
	if e := r.Once(ctx); e == nil || s.success {
		t.Fatal("edge failure hidden")
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestEmptyGroupExplicitlyExcludesEveryone(t *testing.T) {
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Include []map[string]any `json:"include"`
			Exclude []map[string]any `json:"exclude"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Fatal(e)
		}
		if len(body.Include) != 1 || len(body.Exclude) != 1 || body.Include[0]["everyone"] == nil || body.Exclude[0]["everyone"] == nil {
			t.Fatal("empty group can admit a user")
		}
		if r.Header.Get("Authorization") != "Bearer private" {
			t.Fatal("missing scoped auth")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true}`))}, nil
	})}
	c := Cloudflare{AccountID: strings.Repeat("a", 32), Token: "private", GroupNames: map[string]string{strings.Repeat("b", 32): "users"}, Client: client}
	if e := c.Replace(context.Background(), strings.Repeat("b", 32), nil); e != nil {
		t.Fatal(e)
	}
	if e := c.Replace(context.Background(), strings.Repeat("c", 32), nil); e == nil {
		t.Fatal("unrecorded group accepted")
	}
}
