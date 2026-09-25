package broker

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/capability"
)

const fixtureID = "00000000-0000-4000-8000-000000000001"

type authFunc func(context.Context, capability.Claims) error

func (f authFunc) Check(ctx context.Context, c capability.Claims) error { return f(ctx, c) }

type providerFunc func(context.Context, []byte) (*http.Response, error)

type recorderFunc func(context.Context, capability.Claims, string, string, map[string]any) error

func (f recorderFunc) Record(ctx context.Context, c capability.Claims, id, kind string, payload map[string]any) error {
	return f(ctx, c, id, kind, payload)
}

func (f providerFunc) Send(ctx context.Context, b []byte) (*http.Response, error) { return f(ctx, b) }
func setup(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	a := budget.Allocation{Protocol: budget.Protocol, ID: fixtureID, ProjectID: fixtureID, RunID: fixtureID, DeploymentID: fixtureID, ActivationGeneration: 1, LedgerRevision: 1, Amount: 1000000, RequestLimit: 1000000, UserDailyLimit: 1000000, Model: budget.Model, Provider: budget.Provider, Profile: budget.Profile, InputPrice: 100000, OutputPrice: 500000, FeeBasisPoints: 10000, MaxOutputTokens: 64, NotBefore: time.Now().UTC().Add(-time.Minute), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	doc, err := budget.Sign(a, key)
	if err != nil {
		t.Fatal(err)
	}
	run, err := budget.InitializeRun(filepath.Join(dir, "run"), doc, pub, fixtureID, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run.Close() })
	caps, err := capability.Initialize(filepath.Join(dir, "caps"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { caps.DB.Close() })
	c := capability.Claims{AccountID: fixtureID, WorkspaceID: fixtureID, DeploymentID: fixtureID, AuthVersion: 1, WorkspaceGeneration: 1, GrantVersion: 1, ActivationGeneration: 1, Operation: "model", ExpiresAt: time.Now().UTC().Add(time.Minute)}
	token, err := caps.Mint(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(run, caps, authFunc(func(context.Context, capability.Claims) error { return nil }), FakeProvider{}), token
}
func requestBytes(t *testing.T) []byte {
	t.Helper()
	text := "Find a climate product"
	r := Request{Model: budget.Model, MaxTokens: 64, Temperature: "0", Messages: []Message{{Role: "user", Content: &text}}, Tools: ApprovedTools(), ToolChoice: "auto", Stream: true}
	r.StreamOptions.IncludeUsage = true
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func post(t *testing.T, h http.Handler, token string, b []byte, id string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader(b))
	r.URL.Scheme = ""
	r.URL.Host = ""
	r.Header.Set("Authorization", "Bearer "+token)
	if id != "" {
		r.Header.Set("X-Request-ID", id)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestFixedRouteToolContractAndDisclosure(t *testing.T) {
	s, _ := setup(t)
	b := requestBytes(t)
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	raw["provider"] = map[string]any{"only": []string{"unapproved"}, "allow_fallbacks": true}
	raw["messages"] = []any{map[string]any{"role": "user", "content": "key sk-secret /private/data me@example.com product://climate"}, map[string]any{"role": "tool", "content": `{"reference":"product://climate","path":"/private","payload":[1,2],"description":"public metadata"}`}}
	b, _ = json.Marshal(raw)
	out, names, err := Prepare(b, s.Run.Document.Allocation, "sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 11 || strings.Contains(string(out), "parallel_tool_calls") || strings.Contains(string(out), "unapproved") || strings.Contains(string(out), "/private") || strings.Contains(string(out), "payload") || strings.Contains(string(out), "me@example.com") {
		t.Fatal("unsafe outbound", string(out))
	}
	for _, fragment := range []string{`"only":["deepinfra/fp8"]`, `"allow_fallbacks":false`, `"data_collection":"deny"`, `"enabled":false`, `product://climate`} {
		if !strings.Contains(string(out), fragment) {
			t.Fatal("missing fixed policy", fragment)
		}
	}
	raw["model"] = "other"
	b, _ = json.Marshal(raw)
	if _, _, err = Prepare(b, s.Run.Document.Allocation, ""); err == nil {
		t.Fatal("edited model accepted")
	}
	raw["model"] = budget.Model
	raw["parallel_tool_calls"] = true
	b, _ = json.Marshal(raw)
	if _, _, err = Prepare(b, s.Run.Document.Allocation, ""); err == nil {
		t.Fatal("unsupported field accepted")
	}
	delete(raw, "parallel_tool_calls")
	tools := raw["tools"].([]any)
	tools[0].(map[string]any)["function"].(map[string]any)["description"] = "upload files"
	b, _ = json.Marshal(raw)
	if _, _, err = Prepare(b, s.Run.Document.Allocation, ""); err == nil {
		t.Fatal("modified tool schema accepted")
	}
}
func TestBrokerDurableIdempotencyAndNoRetry(t *testing.T) {
	s, token := setup(t)
	var requests atomic.Int32
	s.Provider = providerFunc(func(ctx context.Context, b []byte) (*http.Response, error) {
		requests.Add(1)
		return FakeProvider{}.Send(ctx, b)
	})
	id := UUID()
	first := post(t, s.Workspace(fixtureID), token, requestBytes(t), id)
	if first.Code != 200 || !strings.Contains(first.Body.String(), "[DONE]") {
		t.Fatal(first.Code, first.Body.String())
	}
	second := post(t, s.Workspace(fixtureID), token, requestBytes(t), id)
	if second.Code != 409 || requests.Load() != 1 {
		t.Fatal("duplicate replay", second.Code, requests.Load())
	}
	v, err := s.Run.Lookup(context.Background(), fixtureID, id)
	if err != nil || v.State != "settled" || v.Cost == nil || *v.Cost != 0 {
		t.Fatal(v, err)
	}
	s.Provider = providerFunc(func(context.Context, []byte) (*http.Response, error) {
		requests.Add(1)
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("sk-leak /secret"))}, nil
	})
	id = UUID()
	bad := post(t, s.Workspace(fixtureID), token, requestBytes(t), id)
	if bad.Code != 503 || strings.Contains(bad.Body.String(), "sk-leak") || requests.Load() != 2 {
		t.Fatal("provider error/retry", bad)
	}
	v, err = s.Run.Lookup(context.Background(), fixtureID, id)
	if err != nil || v.State != "unknown" {
		t.Fatal("failed attempt lost", v, err)
	}
}
func TestCrossWorkspaceStaleActivationRevocationAndDispatchCheck(t *testing.T) {
	s, token := setup(t)
	if w := post(t, s.Workspace(UUID()), token, requestBytes(t), ""); w.Code != 403 {
		t.Fatal("cross workspace accepted", w.Code)
	}
	release, err := s.Queue.Acquire(context.Background(), fixtureID)
	if err != nil {
		t.Fatal(err)
	}
	responses := make(chan *httptest.ResponseRecorder, 1)
	go func() { responses <- post(t, s.Workspace(fixtureID), token, requestBytes(t), "") }()
	deadline := time.Now().Add(time.Second)
	for {
		_, waiting := s.Queue.State()
		if waiting == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("request did not queue")
		}
		time.Sleep(time.Millisecond)
	}
	if err = s.Capabilities.Revoke(context.Background(), fixtureID); err != nil {
		t.Fatal(err)
	}
	release()
	select {
	case w := <-responses:
		if w.Code != 403 {
			t.Fatal("revoked queue dispatched", w.Code)
		}
	case <-time.After(time.Second):
		t.Fatal("queue hung")
	}
	s, token = setup(t)
	s.Run.Document.Allocation.ActivationGeneration = 2
	if w := post(t, s.Workspace(fixtureID), token, requestBytes(t), ""); w.Code != 403 {
		t.Fatal("stale activation accepted")
	}
}

type byteReader struct{ b []byte }

func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	p[0] = r.b[0]
	r.b = r.b[1:]
	return 1, nil
}
func TestStreamingSplitUTF8CallsUsageAndFailures(t *testing.T) {
	stream := `data: {"id":"gen-1","model":"deepseek/deepseek-v4.1-flash","provider":"DeepInfra","choices":[{"index":0,"delta":{"content":"é","tool_calls":[{"index":0,"id":"call-1","function":{"name":"help__lookup","arguments":"{\"topic\":"}}]},"finish_reason":null}]}

data: {"id":"gen-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"resolve\"}"}}]},"finish_reason":"tool_calls"}]}

data: {"id":"gen-1","choices":[],"usage":{"cost":0.000001001}}

data: [DONE]

`
	var out bytes.Buffer
	result, err := Relay(&byteReader{[]byte(stream)}, &out, func() {}, map[string]bool{"help__lookup": true})
	if err != nil || result.Cost == nil || *result.Cost != 2 || result.ProviderID != "gen-1" || !strings.Contains(out.String(), "é") || strings.Contains(out.String(), "[DONE]") {
		t.Fatal(result, err, out.String())
	}
	for name, input := range map[string]string{"truncated": strings.Replace(stream, "data: [DONE]\n\n", "", 1), "wrong-provider": strings.Replace(stream, "DeepInfra", "Other", 1), "wrong-model": strings.Replace(stream, budget.Model, "other", 1), "arguments": strings.Replace(stream, `\"resolve\"}`, `\"resolve\"`, 1), "usage": strings.Replace(stream, "0.000001001", "-1", 1), "trailing": stream + "data: {}\n\n", "too-large": strings.Repeat(": comment\n", MaxBody)} {
		t.Run(name, func(t *testing.T) {
			var dst bytes.Buffer
			if _, err := Relay(strings.NewReader(input), &dst, func() {}, map[string]bool{"help__lookup": true}); err == nil {
				t.Fatal("invalid stream accepted")
			}
		})
	}
}
func TestQueueUserAndGlobalLimitsCancellation(t *testing.T) {
	q := NewQueue()
	releases := []func(){}
	for _, user := range []string{"a", "b", "c"} {
		release, err := q.Acquire(context.Background(), user)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	ctx, cancel := context.WithCancel(context.Background())
	blocked := make(chan error, 1)
	go func() { _, err := q.Acquire(ctx, "a"); blocked <- err }()
	time.Sleep(10 * time.Millisecond)
	active, waiting := q.State()
	if active != 3 || waiting != 1 {
		t.Fatal(active, waiting)
	}
	cancel()
	if err := <-blocked; err == nil {
		t.Fatal("cancellation ignored")
	}
	for _, release := range releases {
		release()
	}
	active, waiting = q.State()
	if active != 0 || waiting != 0 {
		t.Fatal("queue slot leaked")
	}
}
func TestHTTPSLoopbackThroughPrivateUnixSocket(t *testing.T) {
	s, token := setup(t)
	dir, err := os.MkdirTemp("/tmp", "feam-broker-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "s")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: s.Workspace(fixtureID)}
	go func() { _ = server.Serve(l) }()
	defer server.Close()
	if err := InitializeLoopbackTrust(dir); err != nil {
		t.Fatal(err)
	}
	if err := InitializeLoopbackTrust(dir); err == nil {
		t.Fatal("certificate replaced implicitly")
	}
	certPath, keyPath, caPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"), filepath.Join(dir, "ca.pem")
	capPath := filepath.Join(dir, "capability")
	if err := os.WriteFile(capPath, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	h, err := AdapterWithCapability(socket, capPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = LoopbackTLS("0.0.0.0:0", certPath, keyPath, h); err == nil {
		t.Fatal("nonloopback TLS accepted")
	}
	tlsServer, tlsListener, err := LoopbackTLS("127.0.0.1:0", certPath, keyPath, h)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = tlsServer.Serve(tlsListener) }()
	defer tlsServer.Close()
	roots := x509.NewCertPool()
	caPEM, _ := os.ReadFile(caPath)
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("invalid CA")
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}, Timeout: time.Second}
	req, _ := http.NewRequest("POST", "https://"+tlsListener.Addr().String()+"/v1/chat/completions", bytes.NewReader(requestBytes(t)))
	req.Header.Set("Authorization", "Bearer workspace-adapter-placeholder")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(b), "[DONE]") {
		t.Fatal(resp.StatusCode, string(b))
	}
	claims, err := s.Capabilities.Check(context.Background(), token, fixtureID, "model")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Capabilities.Revoke(context.Background(), fixtureID); err != nil {
		t.Fatal(err)
	}
	renewed, err := s.Capabilities.Mint(context.Background(), claims)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(capPath+".new", []byte(renewed), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(capPath+".new", capPath); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest("POST", "https://"+tlsListener.Addr().String()+"/v1/chat/completions", bytes.NewReader(requestBytes(t)))
	req.Header.Set("Authorization", "Bearer "+token) // deliberately stale; adapter must replace
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(b), "[DONE]") {
		t.Fatal("capability renewal required restart", resp.StatusCode, string(b))
	}
	req, _ = http.NewRequest("POST", "https://"+tlsListener.Addr().String()+"/arbitrary-proxy", strings.NewReader("{}"))
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatal("arbitrary adapter route accepted")
	}
	t.Run("RustRouterCompatibility", func(t *testing.T) {
		probe := os.Getenv("FEAM_BROKER_PROBE")
		if probe == "" {
			t.Skip("explicit Rust bridge check requires FEAM_BROKER_PROBE; see web_demo/BROKER.md")
		}
		profile := fmt.Sprintf("schema_version = 1\ndefault_profile = \"phase1-demo\"\n[profiles.phase1-demo]\nbackend = \"router\"\nbase_url = %s\nloopback_ca_file = %s\nmodel = \"deepseek/deepseek-v4.1-flash\"\napi_key_env = \"FEAM_BROKER_TEST_CAPABILITY\"\ncontext_policy = \"synthetic-demo\"\nallow_user_text = true\nmax_context_chars = 32000\nmax_output_tokens = 64\nallowed_providers = [\"deepinfra/fp8\"]\nallow_provider_fallbacks = false\n", strconv.Quote("https://"+tlsListener.Addr().String()+"/v1"), strconv.Quote(caPath))
		profilePath := filepath.Join(dir, "agent.toml")
		if err := os.WriteFile(profilePath, []byte(profile), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(probe, profilePath)
		cmd.Env = append(os.Environ(), "FEAM_BROKER_TEST_CAPABILITY="+token)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Rust bridge failed: %v %s", err, output)
		}
		t.Log(strings.TrimSpace(string(output)))
		v, err := s.Run.Lookup(context.Background(), fixtureID, "00000000-0000-4000-8000-000000000002")
		if err != nil || v.State != "settled" {
			t.Fatal("Rust request correlation was not preserved", v, err)
		}
	})
}

func TestCancellationAndActiveRevocationRetainReservation(t *testing.T) {
	for _, kind := range []string{"cancel", "revoke"} {
		t.Run(kind, func(t *testing.T) {
			s, token := setup(t)
			started := make(chan struct{})
			s.Provider = providerFunc(func(ctx context.Context, _ []byte) (*http.Response, error) {
				reader, writer := io.Pipe()
				close(started)
				go func() { <-ctx.Done(); _ = writer.CloseWithError(ctx.Err()) }()
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: reader}, nil
			})
			id := UUID()
			finished := make(chan *httptest.ResponseRecorder, 1)
			go func() { finished <- post(t, s.Workspace(fixtureID), token, requestBytes(t), id) }()
			<-started
			if kind == "cancel" {
				r := httptest.NewRequest("POST", "/v1/requests/"+id+"/cancel", nil)
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				s.Workspace(fixtureID).ServeHTTP(w, r)
				if w.Code != 200 {
					t.Fatal(w.Code)
				}
			} else {
				if err := s.Capabilities.Revoke(context.Background(), fixtureID); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case w := <-finished:
				if strings.Contains(w.Body.String(), "[DONE]") {
					t.Fatal("cancelled stream completed")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("active stream was not cancelled")
			}
			v, err := s.Run.Lookup(context.Background(), fixtureID, id)
			if err != nil || v.State != "unknown" {
				t.Fatal("cancellation released reservation", v, err)
			}
		})
	}
}

func TestCaptureBackpressureStopsDispatchAndCompletion(t *testing.T) {
	s, token := setup(t)
	var requests atomic.Int32
	s.Provider = providerFunc(func(ctx context.Context, b []byte) (*http.Response, error) {
		requests.Add(1)
		return FakeProvider{}.Send(ctx, b)
	})
	s.Events = recorderFunc(func(context.Context, capability.Claims, string, string, map[string]any) error {
		return io.ErrShortWrite
	})
	w := post(t, s.Workspace(fixtureID), token, requestBytes(t), UUID())
	if w.Code != 503 || requests.Load() != 0 || !strings.Contains(w.Body.String(), "telemetry_unavailable") {
		t.Fatal("capture failure dispatched", w.Code, requests.Load())
	}
	s.Events = recorderFunc(func(_ context.Context, _ capability.Claims, _ string, kind string, payload map[string]any) error {
		if kind == "model.usage" {
			return io.ErrShortWrite
		}
		if _, ok := payload["messages"]; ok {
			t.Fatal("raw messages persisted at generic event boundary")
		}
		return nil
	})
	id := UUID()
	w = post(t, s.Workspace(fixtureID), token, requestBytes(t), id)
	if strings.Contains(w.Body.String(), "[DONE]") || !strings.Contains(w.Body.String(), "telemetry_unavailable") {
		t.Fatal("capture failure presented complete", w.Body.String())
	}
	v, err := s.Run.Lookup(context.Background(), fixtureID, id)
	if err != nil || v.State != "settled" {
		t.Fatal("capture failure lost billing", v, err)
	}
}
