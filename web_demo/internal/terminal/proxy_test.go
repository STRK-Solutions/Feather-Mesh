package terminal

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const testCredential = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
const testOrigin = "http://127.0.0.1:18771"

func fixture(t *testing.T, handler http.Handler) *Proxy {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "ttyd.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	p, err := New(testOrigin, socket, testCredential)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func request(path string) *http.Request {
	r := httptest.NewRequest("GET", testOrigin+path, nil)
	r.SetBasicAuth("operator", testCredential)
	return r
}
func TestPrivateConfig(t *testing.T) {
	for _, origin := range []string{"https://example.org", "http://0.0.0.0:18771", "http://localhost:18771", testOrigin + "/", testOrigin + "?production=1"} {
		if _, err := New(origin, "/tmp/ttyd.sock", testCredential); err == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	if _, err := New(testOrigin, "tcp://example.org", testCredential); err == nil {
		t.Fatal("accepted TCP backend")
	}
	if _, err := New(testOrigin, "/tmp/ttyd.sock", "short"); err == nil {
		t.Fatal("accepted weak key")
	}
}
func TestAuthenticationAndRoutes(t *testing.T) {
	p := fixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("terminal")) }))
	cases := []struct {
		name   string
		change func(*http.Request)
		status int
	}{
		{"missing credential", func(r *http.Request) { r.Header.Del("Authorization") }, 401},
		{"fake identity", func(r *http.Request) {
			r.Header.Del("Authorization")
			r.Header.Set("Cf-Access-Authenticated-User-Email", "operator")
			r.Header.Set("X-Test-User", "operator")
		}, 401},
		{"wrong secret", func(r *http.Request) { r.SetBasicAuth("operator", "wrong") }, 401},
		{"host", func(r *http.Request) { r.Host = "evil.example" }, 403},
		{"query commands", func(r *http.Request) { r.URL.RawQuery = "arg=sh" }, 400},
		{"method", func(r *http.Request) { r.Method = "POST" }, 400},
		{"path", func(r *http.Request) { r.URL.Path = "/admin" }, 404},
		{"sibling origin", func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:18772") }, 403},
		{"missing ws origin", func(r *http.Request) { r.URL.Path = "/ws"; r.Header.Set("Upgrade", "websocket") }, 403},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := request("/")
			tt.change(r)
			w := httptest.NewRecorder()
			p.ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("got %d want %d", w.Code, tt.status)
			}
		})
	}
}
func TestHeadersAndResponseBound(t *testing.T) {
	seen := make(chan http.Header, 1)
	p := fixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		w.Header().Set("Set-Cookie", "gateway=stolen")
		w.Header().Set("Location", "https://evil.example")
		w.Header().Set("Cf-Access-Jwt-Assertion", "stolen")
		w.Write([]byte("ok"))
	}))
	r := request("/")
	for _, h := range []string{"Cookie", "Cf-Access-Jwt-Assertion", "X-Forwarded-For", "X-Test-User"} {
		r.Header.Set(h, "private")
	}
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	headers := <-seen
	for _, h := range []string{"Authorization", "Cookie", "Cf-Access-Jwt-Assertion", "X-Forwarded-For", "X-Test-User"} {
		if headers.Get(h) != "" {
			t.Errorf("leaked %s", h)
		}
	}
	for _, h := range []string{"Set-Cookie", "Location", "Cf-Access-Jwt-Assertion"} {
		if w.Header().Get(h) != "" {
			t.Errorf("untrusted %s", h)
		}
	}
	if w.Code != 200 || w.Body.String() != "ok" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("bad response")
	}
	p = fixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "5000000")
		w.WriteHeader(200)
	}))
	w = httptest.NewRecorder()
	p.ServeHTTP(w, request("/"))
	if w.Code != 502 {
		t.Fatal("oversize accepted")
	}
}
func TestWebSocketLimitAndRelease(t *testing.T) {
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == testOrigin }}
	p := fixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			kind, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			if err = c.WriteMessage(kind, data); err != nil {
				return
			}
		}
	}))
	server := httptest.NewServer(p)
	defer server.Close()
	dial := func(origin string) (*websocket.Conn, *http.Response, error) {
		h := request("/").Header
		h.Set("Origin", origin)
		h.Set("Host", strings.TrimPrefix(testOrigin, "http://"))
		return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", h)
	}
	if c, r, err := dial("http://evil.example"); err == nil {
		c.Close()
		t.Fatal("bad origin accepted")
	} else if r.StatusCode != 403 {
		t.Fatal(r.StatusCode)
	}
	a, _, err := dial(testOrigin)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, _, err := dial(testOrigin)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if c, r, err := dial(testOrigin); err == nil {
		c.Close()
		t.Fatal("third connection accepted")
	} else if r.StatusCode != 429 {
		t.Fatal(r.StatusCode)
	}
	if err := a.WriteMessage(websocket.BinaryMessage, []byte("manual FEAM")); err != nil {
		t.Fatal(err)
	}
	_, data, err := a.ReadMessage()
	if err != nil || string(data) != "manual FEAM" {
		t.Fatal("relay failed", err)
	}
	a.Close()
	deadline := time.Now().Add(2 * time.Second)
	for p.active.Load() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	c, _, err := dial(testOrigin)
	if err != nil {
		t.Fatal("slot not released", err)
	}
	c.Close()
}
func TestUnavailableDoesNotLeakSocket(t *testing.T) {
	p, err := New(testOrigin, "/does/not/exist/private.sock", testCredential)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	p.ServeHTTP(w, request("/"))
	body, _ := io.ReadAll(w.Result().Body)
	if w.Code != 502 || strings.Contains(string(body), "private.sock") {
		t.Fatal("unsafe backend error")
	}
}
