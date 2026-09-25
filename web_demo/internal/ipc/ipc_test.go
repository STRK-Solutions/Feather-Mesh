package ipc

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelPeerIdentityAndNoHeaderFallback(t *testing.T) {
	dir, e := os.MkdirTemp("/tmp", "fi-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "service.sock")
	l, e := Listen(socket)
	if e != nil {
		t.Fatal(e)
	}
	handler := RequireUID([]uint32{uint32(os.Getuid())}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("authorized")) }))
	srv := Server(handler)
	go srv.Serve(l)
	defer srv.Close()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://service/", nil)
	resp, e := Client(socket).Do(req)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "authorized" {
		t.Fatal("kernel peer failed")
	}
	r := httptest.NewRequest("GET", "http://service/", nil)
	r.Header.Set("X-UID", "0")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("claimed peer trusted")
	}
	if _, e = Listen(socket); e == nil {
		t.Fatal("existing listener overwritten")
	}
}
func TestStrictBoundedIPC(t *testing.T) {
	for _, body := range []string{`{"unknown":true}`, `{} {}`, strings.Repeat("x", (64<<10)+1)} {
		r := httptest.NewRequest("POST", "http://service/", strings.NewReader(body))
		w := httptest.NewRecorder()
		var v struct{}
		if e := Decode(w, r, &v); e == nil {
			t.Fatal("invalid control body accepted")
		}
	}
}
func TestStaleSocketRecoveryDoesNotRemoveOtherFiles(t *testing.T) {
	dir, e := os.MkdirTemp("/tmp", "fs-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "service.sock")
	l, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		t.Fatal(e)
	}
	l.SetUnlinkOnClose(false)
	l.Close()
	next, e := Listen(path)
	if e != nil {
		t.Fatal("stale own socket not recovered", e)
	}
	next.Close()
	os.WriteFile(path, []byte("preserve"), 0600)
	if _, e = Listen(path); e == nil {
		t.Fatal("regular file removed")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "preserve" {
		t.Fatal("regular file changed")
	}
	os.Remove(path)
	os.Symlink(filepath.Join(dir, "absent"), path)
	if _, e = Listen(path); e == nil {
		t.Fatal("symlink replaced")
	}
}
