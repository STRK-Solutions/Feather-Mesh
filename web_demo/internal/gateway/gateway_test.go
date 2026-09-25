package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/auth"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/db"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/lifecycle"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Signed-token cryptography is exercised by auth tests. This test-only validator
// isolates gateway authorization and is not exposed by production binaries.
type testValidator struct{}

func (testValidator) Validate(_ context.Context, raw, aud string) (auth.Claims, error) {
	parts := strings.Split(raw, "|")
	if len(parts) != 2 || parts[1] != aud {
		return auth.Claims{}, fmt.Errorf("invalid test identity")
	}
	return auth.Claims{Email: parts[0], RegisteredClaims: jwt.RegisteredClaims{Subject: parts[0], IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}, nil
}

type fixture struct {
	g          *Gateway
	accounts   []control.Account
	workspaces []control.Workspace
}

func setup(t *testing.T, socket string) fixture {
	t.Helper()
	d, e := db.Initialize(filepath.Join(t.TempDir(), "control.db"), control.Migrations)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	s := &control.Store{DB: d}
	var roster []control.Enrollment
	for i := 0; i < 9; i++ {
		role := "user"
		if i < 4 {
			role = "admin"
		}
		roster = append(roster, control.Enrollment{Email: fmt.Sprintf("u%d@example.invalid", i), Role: role})
	}
	if e = s.Bootstrap(context.Background(), roster); e != nil {
		t.Fatal(e)
	}
	as, _ := s.Accounts(context.Background())
	cfg := Config{PortalHost: "feam.example.invalid", AdminHost: "admin.example.invalid", Audiences: map[string]string{"feam.example.invalid": "portal", "admin.example.invalid": "admin"}}
	var ws []control.Workspace
	for i := 4; i < 6; i++ {
		w := control.Workspace{ID: control.ID(), OwnerID: as[i].ID, Hostname: "u-" + as[i].ID + ".example.invalid", Socket: socket, Ready: true, Generation: 1, GrantVersion: 1, DeploymentID: control.ID(), ActivationGeneration: 1}
		if e = s.Assign(context.Background(), w); e != nil {
			t.Fatal(e)
		}
		cfg.Audiences[w.Hostname] = "workspace"
		ws = append(ws, w)
	}
	p, _ := s.Policy(context.Background())
	if e = s.SyncResult(context.Background(), p.Revision, true); e != nil {
		t.Fatal(e)
	}
	g, e := New(cfg, s, testValidator{})
	if e != nil {
		t.Fatal(e)
	}
	return fixture{g, as, ws}
}
func request(f fixture, method, host, path, email, aud, body string) *http.Request {
	r := httptest.NewRequest(method, "https://"+host+path, strings.NewReader(body))
	if email != "" {
		r.Header.Set("Cf-Access-Jwt-Assertion", email+"|"+aud)
	}
	return r
}
func TestRoleHostOwnershipAndHeaderNegatives(t *testing.T) {
	f := setup(t, "/run/absent.sock")
	for _, tc := range []struct {
		name, host, path, email, aud string
		status                       int
	}{{"forged email", f.g.Config.PortalHost, "/", "", "", 401}, {"unlisted", f.g.Config.PortalHost, "/", "unknown@example.invalid", "portal", 403}, {"user admin", f.g.Config.AdminHost, "/", f.accounts[4].Email, "admin", 403}, {"wrong aud", f.g.Config.AdminHost, "/", f.accounts[0].Email, "portal", 401}, {"unknown host", "evil.example.invalid", "/", f.accounts[4].Email, "portal", 403}, {"cross user", f.workspaces[1].Hostname, "/", f.accounts[4].Email, "workspace", 403}, {"admin workspace", f.workspaces[0].Hostname, "/", f.accounts[0].Email, "workspace", 403}, {"own portal", f.g.Config.PortalHost, "/", f.accounts[4].Email, "portal", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			r := request(f, "GET", tc.host, tc.path, tc.email, tc.aud, "")
			r.Header.Set("Cf-Access-Authenticated-User-Email", f.accounts[4].Email)
			w := httptest.NewRecorder()
			f.g.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d want %d", w.Code, tc.status)
			}
		})
	}
}
func TestHostOnlyCSRFCookieAndSiblingDenial(t *testing.T) {
	f := setup(t, "/run/absent.sock")
	r := request(f, "GET", f.g.Config.AdminHost, "/", f.accounts[0].Email, "admin", "")
	w := httptest.NewRecorder()
	f.g.ServeHTTP(w, r)
	if w.Header().Get("Referrer-Policy") != "same-origin" {
		t.Fatal("same-origin form submissions must retain their Origin")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing CSRF cookie")
	}
	cookie := cookies[0]
	if cookie.Name != "__Host-feam-csrf" || cookie.Domain != "" || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe cookie")
	}
	form := url.Values{"csrf": {cookie.Value}, "account_id": {f.accounts[4].ID}, "auth_version": {"1"}}
	for _, origin := range []string{"", "null", "https://" + f.g.Config.PortalHost, "https://evil.invalid"} {
		r = request(f, "POST", f.g.Config.AdminHost, "/accounts/disable", f.accounts[0].Email, "admin", form.Encode())
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", origin)
		r.AddCookie(cookie)
		w = httptest.NewRecorder()
		f.g.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("cross origin accepted", origin, w.Code)
		}
	}
	r = request(f, "POST", f.g.Config.AdminHost, "/accounts/disable", f.accounts[0].Email, "admin", form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://"+f.g.Config.AdminHost)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	f.g.ServeHTTP(w, r)
	if w.Code != 303 {
		t.Fatal("valid disable failed", w.Code, w.Body.String())
	}
	a, _ := f.g.Store.Account(context.Background(), f.accounts[4].ID)
	if a.Status != "disabled" || a.AuthVersion != 2 {
		t.Fatal("disable did not revoke")
	}
}
func unixBackend(t *testing.T, h http.Handler) string {
	t.Helper()
	dir, e := os.MkdirTemp("/tmp", "fg-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "b.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	server := &http.Server{Handler: h}
	go server.Serve(l)
	t.Cleanup(func() { server.Close() })
	return path
}
func TestSandboxResponseFiltering(t *testing.T) {
	seen := make(chan http.Header, 1)
	path := unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		w.Header().Set("Set-Cookie", "CF_Authorization=stolen; Domain=.example.invalid")
		w.Header().Set("Cf-Access-Jwt-Assertion", "leak")
		w.Header().Set("Location", "https://admin.example.invalid")
		w.Write([]byte("terminal"))
	}))
	f := setup(t, path)
	r := request(f, "GET", f.workspaces[0].Hostname, "/", f.accounts[4].Email, "workspace", "")
	r.Header.Set("Cookie", "CF_Authorization=credential; __Host-feam-csrf=secret")
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("X-Forwarded-For", "private")
	w := httptest.NewRecorder()
	f.g.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.String() != "terminal" {
		t.Fatal(w.Code, w.Body.String())
	}
	if w.Header().Get("Set-Cookie") != "" || w.Header().Get("Location") != "" || w.Header().Get("Cf-Access-Jwt-Assertion") != "" {
		t.Fatal("sandbox forged auth response")
	}
	headers := <-seen
	for _, key := range []string{"Cookie", "Authorization", "Cf-Access-Jwt-Assertion", "X-Forwarded-For"} {
		if headers.Get(key) != "" {
			t.Fatal("credential reached sandbox", key)
		}
	}
}

func TestWorkspaceTerminalUsesViewportAndRefitsAfterTtydStarts(t *testing.T) {
	const upstream = `<!doctype html><html><head><title>ttyd</title></head><body><div id="terminal-container"></div></body></html>`
	seen := make(chan http.Header, 1)
	path := unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(upstream))
	}))
	f := setup(t, path)
	r := request(f, "GET", f.workspaces[0].Hostname, "/", f.accounts[4].Email, "workspace", "")
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	f.g.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatal("terminal page failed", w.Code)
	}
	if got := (<-seen).Get("Accept-Encoding"); got != "" {
		t.Fatal("compressed terminal HTML bypassed viewport enhancement", got)
	}
	body := w.Body.String()
	for _, expected := range []string{
		`height:100dvh`, `term.options.fontSize=size`, `term.fit()`,
		`ResizeObserver(schedule)`, `devicePixelRatio`, `setTimeout(schedule,1200)`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatal("missing terminal viewport behavior", expected)
		}
	}
	if !strings.HasSuffix(body, "</body></html>") || strings.Count(body, `id="feam-terminal-fit"`) != 1 {
		t.Fatal("terminal page insertion malformed")
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("workspace page lost framing protection")
	}
}

func TestWebSocketOwnershipOriginLimitAndRevocation(t *testing.T) {
	path := unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cf-Access-Jwt-Assertion") != "" || r.Header.Get("Cookie") != "" {
			t.Error("WebSocket credential leaked")
		}
		u := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		c, e := u.Upgrade(w, r, http.Header{"Set-Cookie": []string{"CF_Authorization=stolen"}})
		if e != nil {
			return
		}
		defer c.Close()
		for {
			kind, b, e := c.ReadMessage()
			if e != nil {
				return
			}
			if c.WriteMessage(kind, b) != nil {
				return
			}
		}
	}))
	f := setup(t, path)
	activity := make(chan string, 4)
	f.g.Config.ControllerSocket = unixBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspaces/activity" {
			t.Error("unexpected controller call")
		}
		var in struct {
			WorkspaceID string `json:"workspace_id"`
			Kind        string `json:"kind"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			t.Error("invalid activity")
		}
		activity <- in.Kind
		w.Write([]byte(`{"status":"completed"}`))
	}))
	server := httptest.NewServer(f.g)
	defer server.Close()
	dial := func(owner int, host, origin string) (*websocket.Conn, *http.Response, error) {
		h := http.Header{"Host": []string{host}, "Origin": []string{origin}, "Cf-Access-Jwt-Assertion": []string{f.accounts[owner].Email + "|workspace"}, "Cookie": []string{"CF_Authorization=secret"}}
		return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", h)
	}
	host := f.workspaces[0].Hostname
	for _, tc := range []struct {
		owner  int
		origin string
	}{{5, "https://" + host}, {0, "https://" + host}, {4, "https://" + f.g.Config.PortalHost}} {
		c, resp, e := dial(tc.owner, host, tc.origin)
		if c != nil {
			c.Close()
		}
		if e == nil || resp.StatusCode != 403 {
			t.Fatal("WebSocket admission failed open")
		}
	}
	first, resp, e := dial(4, host, "https://"+host)
	if e != nil {
		t.Fatal(e)
	}
	defer first.Close()
	if resp.Header.Get("Set-Cookie") != "" {
		t.Fatal("sandbox websocket cookie escaped")
	}
	second, _, e := dial(4, host, "https://"+host)
	if e != nil {
		t.Fatal(e)
	}
	defer second.Close()
	third, resp, e := dial(4, host, "https://"+host)
	if third != nil {
		third.Close()
	}
	if e == nil || resp.StatusCode != 429 {
		t.Fatal("connection cap ignored")
	}
	if e = first.WriteMessage(websocket.BinaryMessage, []byte("1resize")); e != nil {
		t.Fatal(e)
	}
	if _, _, e = first.ReadMessage(); e != nil {
		t.Fatal(e)
	}
	select {
	case <-activity:
		t.Fatal("resize counted as input")
	default:
	}
	if e = first.WriteMessage(websocket.BinaryMessage, []byte("0hello")); e != nil {
		t.Fatal(e)
	}
	_, b, e := first.ReadMessage()
	if e != nil || string(b) != "0hello" {
		t.Fatal("terminal echo failed", e)
	}
	select {
	case kind := <-activity:
		if kind != "terminal_input" {
			t.Fatal("wrong activity")
		}
	case <-time.After(time.Second):
		t.Fatal("terminal input not reported")
	}
	if e = f.g.Store.Disable(context.Background(), f.accounts[0].ID, f.accounts[4].ID, 1); e != nil {
		t.Fatal(e)
	}
	f.g.Streams.Check(context.Background())
	first.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, e = first.ReadMessage(); e == nil {
		t.Fatal("disabled stream remained open")
	}
}
func TestConfirmationHashMatchesController(t *testing.T) {
	a := Action{Protocol: "feam.web.v1", RequestID: control.ID(), IdempotencyKey: control.ID(), DeploymentID: control.ID(), ActivationGeneration: 1, WorkspaceID: control.ID(), ExpectedGeneration: 1, ActorID: control.ID(), AuthorizationVersion: 1, Method: "workspace.reset", Parameters: map[string]string{}}
	r := lifecycle.Request{Protocol: a.Protocol, RequestID: a.RequestID, IdempotencyKey: a.IdempotencyKey, DeploymentID: a.DeploymentID, ActivationGeneration: a.ActivationGeneration, WorkspaceID: a.WorkspaceID, ExpectedGeneration: a.ExpectedGeneration, ActorID: a.ActorID, AuthorizationVersion: a.AuthorizationVersion, Method: a.Method}
	if ConfirmationHash(a) != lifecycle.ConfirmationHash(r) {
		t.Fatal("confirmation schema drift")
	}
}
