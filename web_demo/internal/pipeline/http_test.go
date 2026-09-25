package pipeline

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

func TestReleaseIPCNeverTrustsActorHeaders(t *testing.T) {
	m := managerFixture(t)
	request := httptest.NewRequest("POST", "http://pipeline/v1/releases/resolve", strings.NewReader(`{"bundle":"demo-climate","digest":"`+strings.Repeat("a", 64)+`"}`))
	request.Header.Set("X-UID", "123")
	request.Header.Set("X-Role", "admin")
	response := httptest.NewRecorder()
	m.Handler(123).ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatal("claimed service identity accepted")
	}
}

func TestReleaseIPCBoundedClosedAndReadOnly(t *testing.T) {
	m := managerFixture(t)
	dir, err := os.MkdirTemp("/tmp", "fpipe-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "p.sock")
	listener, err := ipc.Listen(socket)
	if err != nil {
		t.Fatal(err)
	}
	// Gateway may be a distinct second allowed UID; no actor JSON is trusted.
	server := ipc.Server(m.Handler(uint32(os.Getuid()+1), uint32(os.Getuid())))
	go server.Serve(listener)
	defer server.Close()
	for _, body := range []string{`{}`, `{"bundle":"demo-climate","digest":"a","path":"/host"}`, `{"bundle":"demo-climate","bundle":"other","digest":"` + strings.Repeat("a", 64) + `"}`, strings.Repeat("a", 65537)} {
		response, err := ipc.Client(socket).Post("http://pipeline/v1/releases/resolve", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatalf("bad request status %d", response.StatusCode)
		}
	}
	response, err := ipc.Client(socket).Post("http://pipeline/v1/releases/resolve", "application/json", strings.NewReader(`{"bundle":"demo-climate","digest":"`+strings.Repeat("a", 64)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 409 {
		t.Fatal("unknown release exposed")
	}
	request, _ := http.NewRequest("POST", "http://pipeline/v1/jobs/prepare", strings.NewReader(`{}`))
	response, err = ipc.Client(socket).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal("unexpected mutation capability")
	}
}
