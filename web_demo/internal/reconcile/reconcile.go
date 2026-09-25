// Package reconcile is the sole writer of exact-email Access group membership.
// Terraform references these group IDs and does not own their contents.
package reconcile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"io"
	"net/http"
	"regexp"
	"sort"
	"time"
)

type Provider interface {
	Replace(context.Context, string, []string) error
}
type Source interface {
	Policy(context.Context) (control.Policy, error)
	Result(context.Context, int64, bool) error
}
type Reconciler struct {
	Source                Source
	Provider              Provider
	UserGroup, AdminGroup string
}

func (r *Reconciler) Once(ctx context.Context) error {
	if r.UserGroup == "" || r.AdminGroup == "" || r.UserGroup == r.AdminGroup {
		return errors.New("distinct recorded groups required")
	}
	p, e := r.Source.Policy(ctx)
	if e != nil {
		return e
	}
	for _, group := range []struct {
		id     string
		emails []string
	}{{r.UserGroup, p.Users}, {r.AdminGroup, p.Admins}} {
		for _, email := range group.emails {
			if _, e = control.Email(email); e != nil {
				return e
			}
		}
		sort.Strings(group.emails)
		if e = r.Provider.Replace(ctx, group.id, group.emails); e != nil {
			_ = r.Source.Result(ctx, p.Revision, false)
			return errors.New("edge reconciliation failed")
		}
	}
	return r.Source.Result(ctx, p.Revision, true)
}

type RemoteSource struct{ Socket string }

func (s RemoteSource) Policy(ctx context.Context) (control.Policy, error) {
	var p control.Policy
	req, e := http.NewRequestWithContext(ctx, "GET", "http://gateway/v1/policy", nil)
	if e != nil {
		return p, e
	}
	resp, e := ipc.Client(s.Socket).Do(req)
	if e != nil {
		return p, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return p, errors.New("policy unavailable")
	}
	e = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&p)
	return p, e
}
func (s RemoteSource) Result(ctx context.Context, revision int64, success bool) error {
	b, _ := json.Marshal(map[string]any{"revision": revision, "success": success})
	req, e := http.NewRequestWithContext(ctx, "POST", "http://gateway/v1/policy/result", bytes.NewReader(b))
	if e != nil {
		return e
	}
	resp, e := ipc.Client(s.Socket).Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("policy changed or result unavailable")
	}
	return nil
}

// Cloudflare config is private and fixed at process start. No browser request
// can choose an account, group, API URL, rule type or credential.
type Cloudflare struct {
	AccountID, Token string
	GroupNames       map[string]string
	Client           *http.Client
}

func (c *Cloudflare) Replace(ctx context.Context, group string, emails []string) error {
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(c.AccountID) || c.Token == "" || c.GroupNames[group] == "" || !regexp.MustCompile(`^[0-9a-f-]{32,36}$`).MatchString(group) {
		return errors.New("invalid recorded membership configuration")
	}
	include := make([]any, 0, len(emails))
	exclude := make([]any, 0)
	for _, email := range emails {
		if _, e := control.Email(email); e != nil {
			return e
		}
		include = append(include, map[string]any{"email": map[string]string{"email": email}})
	}
	if len(include) == 0 { // Explicit impossible membership: include everyone and exclude everyone.
		include = append(include, map[string]any{"everyone": map[string]any{}})
		exclude = append(exclude, map[string]any{"everyone": map[string]any{}})
	}
	body, _ := json.Marshal(map[string]any{"name": c.GroupNames[group], "include": include, "exclude": exclude, "require": []any{}})
	req, e := http.NewRequestWithContext(ctx, "PUT", "https://api.cloudflare.com/client/v4/accounts/"+c.AccountID+"/access/groups/"+group, bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	copyClient := *client
	copyClient.Timeout = 10 * time.Second
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, e := copyClient.Do(req)
	if e != nil {
		return errors.New("membership request unavailable")
	}
	defer resp.Body.Close()
	var result struct {
		Success bool `json:"success"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result) != nil || !result.Success {
		return errors.New("membership replacement rejected")
	}
	return nil
}
