package broker

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Adapter forwards only the model protocol to its installed private Unix socket.
// It does not honor an HTTP proxy, Host override, redirect, or client URL.
func Adapter(socket string) (http.Handler, error) {
	return newAdapter(socket, "")
}

// AdapterWithCapability replaces the caller's bearer with the currently
// provisioned workspace capability on every request. Atomic file renewal needs
// no TUI restart and conveys no authority beyond this mounted workspace.
func AdapterWithCapability(socket, capabilityFile string) (http.Handler, error) {
	if !filepath.IsAbs(capabilityFile) {
		return nil, errors.New("absolute capability file required")
	}
	return newAdapter(socket, capabilityFile)
}

func newAdapter(socket, capabilityFile string) (http.Handler, error) {
	if !filepath.IsAbs(socket) {
		return nil, errors.New("absolute socket required")
	}
	client := &http.Client{Timeout: 120 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socket)
	}, DisableKeepAlives: true, MaxResponseHeaderBytes: 32 << 10, ResponseHeaderTimeout: 120 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.IsAbs() || r.URL.RawQuery != "" || (r.Method != "POST" || r.URL.Path != "/v1/chat/completions") && (r.Method != "GET" || r.URL.Path != "/v1/usage") && !strings.HasPrefix(r.URL.Path, "/v1/requests/") {
			reject(w, 404, "not_found")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, MaxBody)
		req, err := http.NewRequestWithContext(r.Context(), r.Method, "http://unix"+r.URL.Path, r.Body)
		if err != nil {
			reject(w, 400, "invalid_request")
			return
		}
		for _, h := range []string{"Authorization", "Content-Type", "X-Request-ID", "Idempotency-Key"} {
			req.Header.Set(h, r.Header.Get(h))
		}
		if capabilityFile != "" {
			info, err := os.Lstat(capabilityFile)
			// The host may grant this mapped workspace UID read permission via a
			// named ACL (group-class mask 0040). Host validation must also prove
			// group::--- and no unrelated named readers; mode alone cannot.
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0027 != 0 || info.Size() > 128 {
				reject(w, 503, "workspace_capability_unavailable")
				return
			}
			b, err := os.ReadFile(capabilityFile)
			token := strings.TrimSpace(string(b))
			if err != nil || len(token) != 43 {
				reject(w, 503, "workspace_capability_unavailable")
				return
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if req.Header.Get("X-Request-ID") == "" {
			req.Header.Set("X-Request-ID", UUID())
		}
		resp, err := client.Do(req)
		if err != nil {
			reject(w, 503, "provider_transport")
			return
		}
		defer resp.Body.Close()
		for _, h := range []string{"Content-Type", "X-Request-ID"} {
			w.Header().Set(h, resp.Header.Get(h))
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(resp.StatusCode)
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				if _, e := w.Write(buf[:n]); e != nil {
					return
				}
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
			if err != nil {
				return
			}
		}
	}), nil
}
func LoopbackTLS(address, cert, key string, handler http.Handler) (*http.Server, net.Listener, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		return nil, nil, errors.New("adapter requires exact IPv4 loopback")
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, nil, err
	}
	l, err := net.Listen("tcp4", address)
	if err != nil {
		return nil, nil, err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 120 * time.Second, WriteTimeout: 125 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 16 << 10, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}}
	return server, tls.NewListener(l, server.TLSConfig), nil
}

// FakeProvider is explicit offline development mode, never a fallback after an
// upstream failure. It returns no billable usage and exercises the same parser.
type FakeProvider struct{}

func (FakeProvider) Send(ctx context.Context, _ []byte) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body := `data: {"id":"fake-generation","model":"deepseek/deepseek-v4.1-flash","provider":"DeepInfra","choices":[{"index":0,"delta":{"content":"Synthetic broker response.","tool_calls":[{"index":0,"id":"fake-call","function":{"name":"help__lookup","arguments":"{\"topic\":\"resolve\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"cost":0}}

data: [DONE]

`
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}
