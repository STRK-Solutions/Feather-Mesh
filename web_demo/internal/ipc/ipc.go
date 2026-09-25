// Package ipc supplies bounded private HTTP over Unix sockets. Authentication is
// kernel peer identity, never JSON actor fields or browser headers.
package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type peerKey struct{}

func ConnContext(ctx context.Context, c net.Conn) context.Context {
	uid, err := peerUID(c)
	if err != nil {
		return ctx
	}
	return context.WithValue(ctx, peerKey{}, uid)
}
func PeerUID(ctx context.Context) (uint32, bool) {
	uid, ok := ctx.Value(peerKey{}).(uint32)
	return uid, ok
}
func RequireUID(allowed []uint32, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := PeerUID(r.Context())
		for _, a := range allowed {
			if ok && uid == a {
				next.ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, "forbidden service peer", 403)
	})
}
func Client(socket string) *http.Client {
	return &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socket)
	}, MaxResponseHeaderBytes: 32 << 10, ResponseHeaderTimeout: 5 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// Listen preserves active or foreign endpoints. An owner-created stale socket
// can be removed only after refusal, same-inode recheck and parent validation.
// 0660 preserves preprovisioned default ACLs for the explicit service UID map;
// directories must deny all untrusted traversal and group writes.
func Listen(path string) (net.Listener, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("absolute Unix socket required")
	}
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parent.IsDir() || parent.Mode().Perm()&0022 != 0 {
		return nil, errors.New("socket parent must be a trusted non-group-writable directory")
	}
	old, err := os.Lstat(path)
	if err == nil {
		st, ok := old.Sys().(*syscall.Stat_t)
		if !ok || old.Mode()&os.ModeSocket == 0 || st.Uid != uint32(os.Getuid()) {
			return nil, errors.New("socket path is not an owned socket")
		}
		c, dialErr := net.DialTimeout("unix", path, 250*time.Millisecond)
		if dialErr == nil {
			c.Close()
			return nil, errors.New("socket is already active")
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) {
			return nil, errors.New("cannot prove socket is stale")
		}
		current, e := os.Lstat(path)
		if e != nil || !os.SameFile(old, current) {
			return nil, errors.New("socket changed during reconciliation")
		}
		if e = os.Remove(path); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0660); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}

func Server(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ConnContext: ConnContext, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32 << 10}
}
func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
