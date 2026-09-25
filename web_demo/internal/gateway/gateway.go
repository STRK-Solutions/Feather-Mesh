// Package gateway consumes Access identity at exact configured HTTPS origins.
// Runtime and provider credentials are never available to this process.
package gateway

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/auth"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"github.com/gorilla/websocket"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	PortalHost       string            `json:"portal_host"`
	AdminHost        string            `json:"admin_host"`
	Audiences        map[string]string `json:"audiences"`
	ControllerSocket string            `json:"controller_socket"`
	PipelineSocket   string            `json:"pipeline_socket"`
	BrokerSocket     string            `json:"broker_socket"`
	Model            string            `json:"model"`
	Recording        string            `json:"recording"`
	Budget           string            `json:"budget"`
}
type Validator interface {
	Validate(context.Context, string, string) (auth.Claims, error)
}
type Gateway struct {
	Config    Config
	Store     *control.Store
	Validator Validator
	Streams   auth.Streams
	secret    [32]byte
	mu        sync.Mutex
	active    map[string]int
}

func New(c Config, s *control.Store, v Validator) (*Gateway, error) {
	if s == nil || v == nil || c.PortalHost == c.AdminHost || c.Audiences[c.PortalHost] == "" || c.Audiences[c.AdminHost] == "" || c.Audiences[c.PortalHost] == c.Audiences[c.AdminHost] {
		return nil, errors.New("distinct hosts and application audiences required")
	}
	for host, aud := range c.Audiences {
		u, e := url.Parse("https://" + host)
		if e != nil || u.Hostname() != host || u.User != nil || u.Path != "" || strings.ToLower(host) != host || aud == "" {
			return nil, errors.New("invalid host/audience")
		}
	}
	g := &Gateway{Config: c, Store: s, Validator: v, active: map[string]int{}}
	_, e := rand.Read(g.secret[:])
	return g, e
}
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	security(w)
	aud, ok := g.Config.Audiences[r.Host]
	if !ok {
		http.Error(w, "unknown origin", 403)
		return
	}
	if len(r.Header.Values("Cf-Access-Jwt-Assertion")) != 1 {
		http.Error(w, "Access authentication required", 401)
		return
	}
	claims, e := g.Validator.Validate(r.Context(), r.Header.Get("Cf-Access-Jwt-Assertion"), aud)
	if e != nil {
		http.Error(w, "Access authentication required", 401)
		return
	}
	a, e := g.Store.Authenticate(r.Context(), claims.Email, claims.Subject)
	if e != nil {
		http.Error(w, "local authorization denied", 403)
		return
	}
	if r.Host == g.Config.AdminHost && a.Role != "admin" {
		http.Error(w, "admin authorization required", 403)
		return
	}
	if r.Host != g.Config.AdminHost && r.Host != g.Config.PortalHost {
		g.terminal(w, r, a, claims)
		return
	}
	if r.URL.RawQuery != "" || r.URL.RawPath != "" {
		http.Error(w, "invalid route", 400)
		return
	}
	if r.Method == http.MethodPost {
		if !g.csrf(r, a) || claims.IssuedAt == nil || time.Since(claims.IssuedAt.Time) > time.Hour {
			http.Error(w, "fresh same-origin confirmation required", 403)
			return
		}
		g.mutate(w, r, a)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if r.URL.Path != "/" && !(r.Host == g.Config.AdminHost && strings.HasPrefix(r.URL.Path, "/pipeline/review/")) {
		http.NotFound(w, r)
		return
	}
	token := g.sign(csrfValue{Account: a.ID, Version: a.AuthVersion, Host: r.Host, Expires: time.Now().Add(time.Hour).Unix(), Nonce: control.ID()})
	http.SetCookie(w, &http.Cookie{Name: "__Host-feam-csrf", Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 3600})
	if r.URL.Path != "/" {
		g.pipelineReview(w, r, a, token)
		return
	}
	data := page{Account: a, CSRF: token, Config: g.Config}
	data.BrokerStatus = g.brokerStatus(r.Context())
	if a.Role == "admin" && r.Host == g.Config.AdminHost {
		data.Accounts, e = g.Store.Accounts(r.Context())
		if e == nil {
			data.Policy, e = g.Store.Policy(r.Context())
		}
		if e == nil {
			data.Jobs, e = g.Store.Jobs(r.Context())
		}
		if e == nil {
			data.Workspaces, e = g.workspacePages(r.Context(), data.Accounts)
		}
		data.PipelineJobs, data.PipelineAvailable = g.pipelineJobs(r.Context())
	} else {
		ws, err := g.Store.WorkspaceForOwner(r.Context(), a.ID)
		if err == nil {
			data.Workspace = &ws
			data.LifecycleState, data.WarningAt, data.RetentionNotified, data.RetentionDeleteAfter = g.workspaceStatus(r.Context(), ws)
		}
	}
	if e != nil {
		http.Error(w, "control store unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = home.Execute(w, data)
}
func security(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Same-origin forms must retain Origin for CSRF validation. no-referrer
	// makes browsers send Origin: null on ordinary form POSTs.
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; form-action 'self'; base-uri 'none'")
}

type csrfValue struct {
	Account string
	Version int64
	Host    string
	Expires int64
	Nonce   string
}

func (g *Gateway) sign(v any) string {
	b, _ := json.Marshal(v)
	m := hmac.New(sha256.New, g.secret[:])
	m.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func (g *Gateway) unsign(s string, v any) bool {
	if len(s) > 16384 {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return false
	}
	b, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return false
	}
	mac, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return false
	}
	m := hmac.New(sha256.New, g.secret[:])
	m.Write(b)
	return hmac.Equal(mac, m.Sum(nil)) && json.Unmarshal(b, v) == nil
}
func (g *Gateway) csrf(r *http.Request, a control.Account) bool {
	if r.Header.Get("Origin") != "https://"+r.Host || len(r.Header.Values("Origin")) != 1 {
		return false
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 64<<10)
	if e := r.ParseForm(); e != nil {
		return false
	}
	cookies := r.CookiesNamed("__Host-feam-csrf")
	if len(cookies) != 1 || cookies[0].Value != r.PostForm.Get("csrf") {
		return false
	}
	var v csrfValue
	return g.unsign(cookies[0].Value, &v) && v.Account == a.ID && v.Version == a.AuthVersion && v.Host == r.Host && v.Expires > time.Now().Unix()
}

type Action struct {
	Protocol             string            `json:"protocol"`
	RequestID            string            `json:"request_id"`
	IdempotencyKey       string            `json:"idempotency_key"`
	DeploymentID         string            `json:"deployment_id"`
	ActivationGeneration int64             `json:"activation_generation"`
	WorkspaceID          string            `json:"workspace_id"`
	ExpectedGeneration   int64             `json:"expected_generation"`
	ActorID              string            `json:"actor_id"`
	AuthorizationVersion int64             `json:"authorization_version"`
	Method               string            `json:"method"`
	Parameters           map[string]string `json:"parameters"`
}
type confirmation struct {
	Action  Action
	Expires int64
}

func ConfirmationHash(a Action) string {
	a.Parameters = map[string]string{}
	b, _ := json.Marshal(a)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (g *Gateway) mutate(w http.ResponseWriter, r *http.Request, a control.Account) {
	switch r.URL.Path {
	case "/accounts/invite":
		if r.Host != g.Config.AdminHost {
			http.Error(w, "forbidden", 403)
			return
		}
		_, e := g.Store.Invite(r.Context(), a.ID, control.Enrollment{Email: r.PostForm.Get("email"), Role: r.PostForm.Get("role")})
		if e != nil {
			http.Error(w, "account was not created", 409)
			return
		}
		http.Redirect(w, r, "/", 303)
	case "/accounts/disable":
		if r.Host != g.Config.AdminHost {
			http.Error(w, "forbidden", 403)
			return
		}
		version, _ := strconv.ParseInt(r.PostForm.Get("auth_version"), 10, 64)
		target := r.PostForm.Get("account_id")
		if e := g.Store.Disable(r.Context(), a.ID, target, version); e != nil {
			http.Error(w, "account changed; reload", 409)
			return
		}
		g.Streams.Revoke(target)
		http.Redirect(w, r, "/", 303)
	case "/workspace/action":
		g.action(w, r, a)
	case "/datasets/grant":
		g.grant(w, r, a)
	case "/pipeline/plan", "/pipeline/enqueue", "/pipeline/approve", "/pipeline/promote", "/pipeline/reconcile", "/pipeline/withdrawal-plan", "/pipeline/prune":
		g.pipelineAction(w, r, a)
	default:
		http.NotFound(w, r)
	}
}
func (g *Gateway) action(w http.ResponseWriter, r *http.Request, a control.Account) {
	var action Action
	confirmed := false
	if token := r.PostForm.Get("confirmation"); token != "" {
		var c confirmation
		if !g.unsign(token, &c) || c.Expires < time.Now().Unix() || c.Action.ActorID != a.ID || c.Action.AuthorizationVersion != a.AuthVersion {
			http.Error(w, "confirmation expired", 403)
			return
		}
		action = c.Action
		confirmed = true
	} else {
		id := r.PostForm.Get("workspace_id")
		generation, e := strconv.ParseInt(r.PostForm.Get("generation"), 10, 64)
		if e != nil {
			http.Error(w, "invalid generation", 400)
			return
		}
		ws, e := g.Store.Workspace(r.Context(), id)
		if e != nil {
			http.Error(w, "workspace denied", 403)
			return
		}
		action = Action{Protocol: "feam.web.v1", RequestID: control.ID(), IdempotencyKey: control.ID(), DeploymentID: ws.DeploymentID, ActivationGeneration: ws.ActivationGeneration, WorkspaceID: id, ExpectedGeneration: generation, ActorID: a.ID, AuthorizationVersion: a.AuthVersion, Method: r.PostForm.Get("action"), Parameters: map[string]string{}}
	}
	ws, e := g.Store.Workspace(r.Context(), action.WorkspaceID)
	adminAction := r.Host == g.Config.AdminHost && a.Role == "admin" && (action.Method == "workspace.stop" || action.Method == "workspace.reset" || action.Method == "workspace.delete")
	if e != nil || ws.OwnerID != a.ID && !adminAction {
		http.Error(w, "workspace denied", 403)
		return
	}
	if ws.Generation != action.ExpectedGeneration {
		http.Error(w, "workspace changed; reload", 409)
		return
	}
	switch action.Method {
	case "workspace.start", "workspace.stop":
	case "workspace.reset", "workspace.delete":
		if !confirmed {
			if !g.registerConfirmation(r.Context(), action) {
				http.Error(w, "controller confirmation unavailable", 503)
				return
			}
			token := g.sign(confirmation{action, time.Now().Add(5 * time.Minute).Unix()})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = review.Execute(w, map[string]string{"CSRF": r.PostForm.Get("csrf"), "Confirmation": token, "Action": action.Method, "Workspace": ws.ID})
			return
		}
		action.Parameters["confirmation_hash"] = ConfirmationHash(action)
	default:
		http.Error(w, "unsupported action", 400)
		return
	}
	if g.Config.ControllerSocket == "" {
		http.Error(w, "controller unavailable", 503)
		return
	}
	b, _ := json.Marshal(action)
	req, e := http.NewRequestWithContext(r.Context(), "POST", "http://controller/v1/workspaces/action", bytes.NewReader(b))
	if e != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := ipc.Client(g.Config.ControllerSocket).Do(req)
	if e != nil {
		http.Error(w, "lifecycle outcome unknown; inspect jobs before retry", 503)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		http.Error(w, "lifecycle admission rejected; inspect jobs", resp.StatusCode)
		return
	}
	var result struct {
		Protocol  string `json:"protocol"`
		RequestID string `json:"request_id"`
		Status    string `json:"status"`
		JobID     string `json:"job_id"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result); e != nil {
		http.Error(w, "lifecycle outcome unknown; inspect jobs", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_ = json.NewEncoder(w).Encode(result)
}
func filtered(in http.Header, names ...string) http.Header {
	out := make(http.Header)
	for _, name := range names {
		for _, v := range in.Values(name) {
			out.Add(name, v)
		}
	}
	return out
}
func (g *Gateway) terminal(w http.ResponseWriter, r *http.Request, a control.Account, c auth.Claims) {
	ws, e := g.Store.WorkspaceForHost(r.Context(), r.Host)
	if e != nil || ws.OwnerID != a.ID || !ws.Ready {
		http.Error(w, "workspace denied", 403)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" || r.URL.RawQuery != "" || r.URL.RawPath != "" || r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
		http.Error(w, "invalid terminal request", 400)
		return
	}
	switch r.URL.Path {
	case "/", "/ws", "/token", "/favicon.ico":
	default:
		http.NotFound(w, r)
		return
	}
	upgrade := strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
	if r.URL.Path == "/ws" {
		if !upgrade || len(r.Header.Values("Origin")) != 1 || r.Header.Get("Origin") != "https://"+r.Host {
			http.Error(w, "invalid WebSocket origin", 403)
			return
		}
		g.mu.Lock()
		if g.active[ws.ID] >= 2 {
			g.mu.Unlock()
			http.Error(w, "two terminal connections allowed", 429)
			return
		}
		g.active[ws.ID]++
		g.mu.Unlock()
		defer func() { g.mu.Lock(); g.active[ws.ID]--; g.mu.Unlock() }()
	} else if upgrade || r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "https://"+r.Host {
		http.Error(w, "invalid origin", 403)
		return
	}
	if upgrade {
		g.websocket(w, r, a, ws, c)
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'none'")
	proxy := &httputil.ReverseProxy{Rewrite: func(p *httputil.ProxyRequest) {
		p.SetURL(&url.URL{Scheme: "http", Host: "terminal"})
		p.Out.Host = r.Host
		p.Out.Header = filtered(p.Out.Header, "Accept", "Accept-Encoding", "Origin", "Connection", "Upgrade", "Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Protocol")
		if r.URL.Path == "/" {
			p.Out.Header.Del("Accept-Encoding") // root HTML is enhanced without decoding untrusted bytes
		}
	}, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", ws.Socket)
	}, ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 32 << 10, DisableKeepAlives: true, DisableCompression: true}, ModifyResponse: func(resp *http.Response) error {
		resp.Header = filtered(resp.Header, "Content-Type", "Content-Encoding", "Connection", "Upgrade", "Sec-WebSocket-Accept", "Sec-WebSocket-Protocol")
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			if err := enhanceTerminalPage(resp); err != nil {
				return err
			}
		}
		if resp.StatusCode != 101 {
			if resp.ContentLength > 4<<20 {
				return errors.New("response too large")
			}
			resp.Body = limitedBody{io.LimitReader(resp.Body, 4<<20), resp.Body}
		}
		return nil
	}, ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) { http.Error(w, "terminal unavailable", 502) }}
	proxy.ServeHTTP(w, r)
}

type limitedBody struct {
	io.Reader
	io.Closer
}

func (g *Gateway) websocket(w http.ResponseWriter, r *http.Request, a control.Account, ws control.Workspace, c auth.Claims) {
	dialer := websocket.Dialer{NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", ws.Socket)
	}, HandshakeTimeout: 5 * time.Second, Subprotocols: websocket.Subprotocols(r)}
	backend, response, err := dialer.DialContext(r.Context(), "ws://"+r.Host+"/ws", http.Header{"Origin": []string{"https://" + r.Host}})
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		http.Error(w, "terminal unavailable", 502)
		return
	}
	defer backend.Close()
	upgrader := websocket.Upgrader{CheckOrigin: func(req *http.Request) bool { return req.Header.Get("Origin") == "https://"+r.Host }, Subprotocols: []string{backend.Subprotocol()}, HandshakeTimeout: 5 * time.Second}
	frontend, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer frontend.Close()
	frontend.SetReadLimit(64 << 10)
	backend.SetReadLimit(1 << 20)
	untrack := g.Streams.Track(a.ID, connectionPair{frontend, backend}, c.ExpiresAt.Time, func(ctx context.Context) bool {
		_, e := g.Store.Authorize(ctx, control.Check{AccountID: a.ID, AuthVersion: a.AuthVersion, WorkspaceID: ws.ID, WorkspaceGeneration: ws.Generation, GrantVersion: ws.GrantVersion, DeploymentID: ws.DeploymentID, ActivationGeneration: ws.ActivationGeneration})
		return e == nil
	})
	defer untrack()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	activity := make(chan struct{}, 1)
	go g.activity(ctx, ws.ID, activity)
	done := make(chan struct{}, 2)
	copyFrames := func(dst, src *websocket.Conn, input bool) {
		defer func() { done <- struct{}{} }()
		for {
			kind, data, e := src.ReadMessage()
			if e != nil {
				return
			}
			if input && kind == websocket.BinaryMessage && len(data) > 1 && data[0] == '0' {
				select {
				case activity <- struct{}{}:
				default:
				}
			}
			_ = dst.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if dst.WriteMessage(kind, data) != nil {
				return
			}
		}
	}
	go copyFrames(backend, frontend, true)
	go copyFrames(frontend, backend, false)
	<-done
	_ = frontend.Close()
	_ = backend.Close()
	<-done
}

type connectionPair struct{ frontend, backend *websocket.Conn }

func (c connectionPair) Close() error { _ = c.frontend.Close(); return c.backend.Close() }

// ttyd input frames start with ASCII '0'; resize, pings and authentication are
// deliberately excluded. Coalesce input to at most one activity IPC per second.
func (g *Gateway) activity(ctx context.Context, id string, ch <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
		}
		if g.Config.ControllerSocket != "" {
			b, _ := json.Marshal(map[string]string{"workspace_id": id, "kind": "terminal_input"})
			r, e := http.NewRequestWithContext(ctx, "POST", "http://controller/v1/workspaces/activity", bytes.NewReader(b))
			if e == nil {
				resp, e := ipc.Client(g.Config.ControllerSocket).Do(r)
				if e == nil {
					_ = resp.Body.Close()
				}
			}
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (g *Gateway) RunRevocations(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		g.revoke(ctx)
		g.reconfigureGrants(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (g *Gateway) revoke(ctx context.Context) {
	accounts, e := g.Store.PendingRevocations(ctx)
	if e != nil {
		return
	}
	for _, a := range accounts {
		g.Streams.Revoke(a.ID)
		if g.Config.ControllerSocket == "" {
			continue
		}
		b, _ := json.Marshal(map[string]any{"account_id": a.ID, "auth_version": a.AuthVersion})
		r, e := http.NewRequestWithContext(ctx, "POST", "http://controller/v1/accounts/revoke", bytes.NewReader(b))
		if e != nil {
			continue
		}
		resp, e := ipc.Client(g.Config.ControllerSocket).Do(r)
		if e != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == 200 {
			_ = g.Store.RevokeComplete(ctx, a.ID, a.AuthVersion)
		}
	}
}

func (g *Gateway) registerConfirmation(ctx context.Context, a Action) bool {
	if g.Config.ControllerSocket == "" {
		return false
	}
	a.Parameters["confirmation_hash"] = ConfirmationHash(a)
	b, _ := json.Marshal(a)
	r, e := http.NewRequestWithContext(ctx, "POST", "http://controller/v1/workspaces/confirm", bytes.NewReader(b))
	if e != nil {
		return false
	}
	resp, e := ipc.Client(g.Config.ControllerSocket).Do(r)
	if e != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200 || resp.StatusCode == 201
}
