package broker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

type operationsFunc func(context.Context, string, string, bool) error

func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "feam-broker-control-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func (f operationsFunc) Set(ctx context.Context, workspace, request string, active bool) error {
	return f(ctx, workspace, request, active)
}

func TestDrainCancelsActiveAndQueuedWithoutDispatchOrBudgetRelease(t *testing.T) {
	s, token := setup(t)
	s.ReceiptFile = filepath.Join(t.TempDir(), "receipt.json")
	if err := os.Chmod(filepath.Dir(s.ReceiptFile), 0700); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	var calls atomic.Int32
	s.Provider = providerFunc(func(ctx context.Context, _ []byte) (*http.Response, error) {
		calls.Add(1)
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	var operationsMu sync.Mutex
	var operations []bool
	s.Operations = operationsFunc(func(_ context.Context, workspace, request string, active bool) error {
		if workspace != fixtureID || request != fixtureID {
			t.Errorf("wrong activity identity %s %s", workspace, request)
		}
		operationsMu.Lock()
		operations = append(operations, active)
		operationsMu.Unlock()
		return nil
	})
	b := requestBytes(t)
	var requests sync.WaitGroup
	requests.Add(2)
	go func() { defer requests.Done(); post(t, s.Workspace(fixtureID), token, b, fixtureID) }()
	<-started
	go func() {
		defer requests.Done()
		post(t, s.Workspace(fixtureID), token, b, "00000000-0000-4000-8000-000000000002")
	}()
	deadline := time.Now().Add(time.Second)
	for {
		_, queued := s.Queue.State()
		if queued == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("request did not queue")
		}
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	receipt, err := s.Drain(ctx)
	requests.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || receipt.RequestCount != 1 || receipt.UnknownRequests != 1 || !receipt.Usage.Paused || receipt.Usage.Unknown <= 0 || receipt.Usage.Reserved != 0 || !receipt.RequiresReconcile {
		t.Fatalf("drain released/replayed accounting: %+v calls=%d", receipt, calls.Load())
	}
	if len(operations) != 2 || !operations[0] || operations[1] {
		t.Fatalf("queue refreshed activity or lease not cleared: %v", operations)
	}
	info, err := os.Stat(s.ReceiptFile)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("receipt not private", err)
	}
	var persisted budget.DrainReceipt
	data, _ := os.ReadFile(s.ReceiptFile)
	if json.Unmarshal(data, &persisted) != nil || persisted.AllocationSHA256 != s.Run.Document.Digest() || persisted.Usage != receipt.Usage {
		t.Fatal("receipt not bound to allocation")
	}
	if w := post(t, s.Workspace(fixtureID), token, b, "00000000-0000-4000-8000-000000000003"); w.Code != 503 {
		t.Fatal("drained run accepted a new request", w.Code)
	}
	second, err := s.Drain(ctx)
	if err != nil || second.Usage != receipt.Usage {
		t.Fatal("repeated drain changed cost", second, err)
	}
}

func TestActivityFailurePreventsProviderAndDrainTimeoutNeverWritesReceipt(t *testing.T) {
	s, token := setup(t)
	var calls atomic.Int32
	s.Provider = providerFunc(func(context.Context, []byte) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("not called")
	})
	s.Operations = operationsFunc(func(context.Context, string, string, bool) error { return errOperation })
	w := post(t, s.Workspace(fixtureID), token, requestBytes(t), fixtureID)
	if w.Code != 503 || calls.Load() != 0 {
		t.Fatal("activity failure dispatched", w.Code, calls.Load())
	}
	s.ReceiptFile = filepath.Join(t.TempDir(), "receipt.json")
	if err := os.Chmod(filepath.Dir(s.ReceiptFile), 0700); err != nil {
		t.Fatal(err)
	}
	finished, err := s.admit(func() {})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = s.Drain(ctx)
	finished()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("unfinished drain acknowledged", err)
	}
	if _, err = os.Stat(s.ReceiptFile); !os.IsNotExist(err) {
		t.Fatal("receipt written before handlers finished")
	}
	usage, err := s.Run.Usage(context.Background(), "")
	if err != nil || !usage.Paused {
		t.Fatal("timeout removed durable pause", usage, err)
	}
}

func TestControlStatusPeerCannotDrainAndActivityUsesFixedUnixRoute(t *testing.T) {
	s, _ := setup(t)
	dir := shortDir(t)
	path := filepath.Join(dir, "control.sock")
	l, err := ipc.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	server := ipc.Server(s.Control(uint32(os.Getuid()+1), uint32(os.Getuid())))
	go func() { _ = server.Serve(l) }()
	defer server.Close()
	client := ipc.Client(path)
	response, err := client.Get("http://unix/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	var status map[string]any
	err = json.NewDecoder(response.Body).Decode(&status)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || status["global_concurrency"] != float64(3) || len(status) != 6 {
		t.Fatal("aggregate status invalid", status, err)
	}
	response, err = client.Post("http://unix/v1/run/drain", "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("read-only peer drained run", response.StatusCode)
	}
	operationPath := filepath.Join(dir, "operations.sock")
	l2, err := ipc.Listen(operationPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Close()
	var calls atomic.Int32
	operations := ipc.Server(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Workspace string `json:"workspace_id"`
			Request   string `json:"request_id"`
			Active    bool   `json:"active"`
		}
		if r.Method != "POST" || r.URL.Path != "/v1/workspaces/operation" || ipc.Decode(w, r, &v) != nil || v.Workspace != fixtureID || v.Request != fixtureID {
			t.Error("operation client sent wrong route or authority")
			http.Error(w, "invalid", 400)
			return
		}
		if calls.Add(1) == 2 {
			http.Error(w, "denied", 403)
		}
	}))
	go func() { _ = operations.Serve(l2) }()
	defer operations.Close()
	op := NewControlOperations(operationPath)
	if err = op.Set(context.Background(), fixtureID, fixtureID, true); err != nil {
		t.Fatal(err)
	}
	if err = op.Set(context.Background(), fixtureID, fixtureID, false); err == nil || calls.Load() != 2 {
		t.Fatal("operation failure ignored or retried", err, calls.Load())
	}
}
