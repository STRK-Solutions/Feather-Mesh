// Package terminal implements the operator-only W1 terminal proxy.
// The production identity gateway is a separate W2 entrypoint; no test identity
// headers, public listener, runtime socket or lifecycle actions exist here.
package terminal

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type Proxy struct {
	origin   string
	host     string
	secret   [32]byte
	active   atomic.Int32
	upstream *httputil.ReverseProxy
}

// New only accepts a loopback browser origin and a filesystem Unix backend.
// The credential is an operator-generated 256-bit hexadecimal secret, never a
// claimed participant identity. It must be kept outside the container.
func New(origin, socket, credential string) (*Proxy, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("private terminal requires an exact http://127.0.0.1:port origin")
	}
	if !strings.HasPrefix(socket, "/") || strings.ContainsAny(socket, "\x00\r\n") || len(credential) != 64 || strings.Trim(credential, "0123456789abcdef") != "" {
		return nil, errors.New("invalid Unix backend or operator credential")
	}
	p := &Proxy{origin: origin, host: u.Host, secret: sha256.Sum256([]byte(credential))}
	p.upstream = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(&url.URL{Scheme: "http", Host: "terminal"})
			r.Out.Host = p.host
			// Allowlist instead of trying to enumerate every identity header.
			r.Out.Header = filtered(r.Out.Header, []string{"Accept", "Accept-Encoding", "Origin", "Connection", "Upgrade", "Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Protocol"})
		},
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socket)
			},
			ResponseHeaderTimeout:  10 * time.Second,
			MaxResponseHeaderBytes: 32 << 10,
			DisableCompression:     true,
			MaxIdleConnsPerHost:    2,
			IdleConnTimeout:        30 * time.Second,
		},
		ModifyResponse: func(r *http.Response) error {
			r.Header = filtered(r.Header, []string{"Content-Type", "Content-Encoding", "Upgrade", "Connection", "Sec-WebSocket-Accept", "Sec-WebSocket-Protocol"})
			r.Header.Set("Cache-Control", "no-store")
			r.Header.Set("X-Content-Type-Options", "nosniff")
			r.Header.Set("Referrer-Policy", "no-referrer")
			r.Header.Set("X-Frame-Options", "DENY")
			if r.StatusCode != http.StatusSwitchingProtocols {
				if r.ContentLength > 4<<20 {
					return errors.New("terminal response exceeds limit")
				}
				r.Body = &boundedBody{Reader: io.LimitReader(r.Body, 4<<20), Closer: r.Body}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "terminal unavailable", http.StatusBadGateway)
		},
	}
	return p, nil
}

type boundedBody struct {
	io.Reader
	io.Closer
}

func filtered(in http.Header, keys []string) http.Header {
	out := make(http.Header)
	for _, key := range keys {
		for _, value := range in.Values(key) {
			out.Add(key, value)
		}
	}
	return out
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Host != p.host {
		http.Error(w, "unknown host", http.StatusForbidden)
		return
	}
	name, password, ok := r.BasicAuth()
	hash := sha256.Sum256([]byte(password))
	if !ok || name != "operator" || subtle.ConstantTimeCompare(hash[:], p.secret[:]) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="FEAM private W1", charset="UTF-8"`)
		http.Error(w, "operator authentication required", http.StatusUnauthorized)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" || r.URL.RawQuery != "" || r.URL.RawPath != "" || r.ContentLength > 0 || len(r.TransferEncoding) != 0 {
		http.Error(w, "unsupported request", http.StatusBadRequest)
		return
	}
	if r.URL.Path != "/" && r.URL.Path != "/ws" && r.URL.Path != "/token" && r.URL.Path != "/favicon.ico" {
		http.NotFound(w, r)
		return
	}
	upgrade := strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
	if r.URL.Path == "/ws" {
		if !upgrade || r.Method != "GET" || r.Header.Get("Origin") != p.origin || len(r.Header.Values("Origin")) != 1 {
			http.Error(w, "invalid terminal origin", http.StatusForbidden)
			return
		}
		if p.active.Add(1) > 2 {
			p.active.Add(-1)
			http.Error(w, "two terminal connections allowed", http.StatusTooManyRequests)
			return
		}
		defer p.active.Add(-1)
	} else if upgrade || r.Header.Get("Origin") != "" && r.Header.Get("Origin") != p.origin {
		http.Error(w, "invalid origin", http.StatusForbidden)
		return
	}
	p.upstream.ServeHTTP(&deadlineWriter{ResponseWriter: w}, r)
}

// net/http does not retain HTTP deadlines after a WebSocket hijack. Bound the
// private browser connection even if its upstream becomes silent.
type deadlineWriter struct{ http.ResponseWriter }

func (w *deadlineWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *deadlineWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, b, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		err = c.SetDeadline(time.Now().Add(time.Hour))
		if err != nil {
			c.Close()
		}
	}
	return c, b, err
}
