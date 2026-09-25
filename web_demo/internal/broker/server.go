package broker

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/capability"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
)

type Authority interface {
	Check(context.Context, capability.Claims) error
}
type ControlAuthority struct{ Client *http.Client }

func NewControlAuthority(socket string) ControlAuthority { return ControlAuthority{ipc.Client(socket)} }
func (a ControlAuthority) Check(ctx context.Context, c capability.Claims) error {
	claims := map[string]any{"account_id": c.AccountID, "auth_version": c.AuthVersion, "workspace_id": c.WorkspaceID, "workspace_generation": c.WorkspaceGeneration, "grant_version": c.GrantVersion, "deployment_id": c.DeploymentID, "activation_generation": c.ActivationGeneration}
	b, _ := json.Marshal(claims)
	req, err := http.NewRequestWithContext(ctx, "POST", "http://unix/v1/authorize", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return capability.ErrDenied
	}
	return nil
}

type Provider interface {
	Send(context.Context, []byte) (*http.Response, error)
}

// EventRecorder is a separate bounded research channel. A failed recorder must
// stop admission; authoritative billing remains in Run even when capture fails.
type EventRecorder interface {
	Record(context.Context, capability.Claims, string, string, map[string]any) error
}
type OpenRouter struct {
	key    string
	client *http.Client
}

func NewOpenRouter(key string) (*OpenRouter, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("missing broker credential")
	}
	return &OpenRouter{key, &http.Client{Timeout: 120 * time.Second, Transport: &http.Transport{ResponseHeaderTimeout: 20 * time.Second, MaxResponseHeaderBytes: 32 << 10, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (p *OpenRouter) Send(ctx context.Context, b []byte) (*http.Response, error) {
	r, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+p.key)
	r.Header.Set("Content-Type", "application/json")
	return p.client.Do(r)
}

type Server struct {
	Run          *budget.Run
	Capabilities *capability.Store
	Authority    Authority
	Provider     Provider
	Queue        *Queue
	Secret       string
	Events       EventRecorder
	Operations   Operations
	ReceiptFile  string
	mu           sync.Mutex
	active       map[string]context.CancelFunc
	pending      map[uint64]context.CancelFunc
	next         uint64
	draining     bool
	requests     sync.WaitGroup
	drainMu      sync.Mutex
}

func NewServer(run *budget.Run, caps *capability.Store, auth Authority, p Provider) *Server {
	return &Server{Run: run, Capabilities: caps, Authority: auth, Provider: p, Queue: NewQueue(), active: map[string]context.CancelFunc{}, pending: map[uint64]context.CancelFunc{}}
}
func UUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func jsonReply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func reject(w http.ResponseWriter, status int, code string) {
	jsonReply(w, status, map[string]any{"error": map[string]string{"code": code, "message": code}})
}
func (s *Server) authorize(ctx context.Context, token, workspace string) (capability.Claims, error) {
	c, err := s.Capabilities.Check(ctx, token, workspace, "model")
	if err != nil {
		return c, err
	}
	a := s.Run.Document.Allocation
	if c.DeploymentID != a.DeploymentID || c.ActivationGeneration != a.ActivationGeneration || !time.Now().Before(a.ExpiresAt) {
		return c, capability.ErrDenied
	}
	if s.Authority == nil {
		return c, capability.ErrDenied
	}
	return c, s.Authority.Check(ctx, c)
}

// Workspace binds the mounted endpoint to a single workspace. The runtime
// controller installs only that directory, never a shared multiplexing socket.
func (s *Server) Workspace(workspace string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !budget.ValidID(workspace) || r.URL.RawQuery != "" || r.URL.IsAbs() {
			reject(w, 400, "invalid_request")
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims, err := s.authorize(r.Context(), token, workspace)
		if err != nil {
			reject(w, 403, "forbidden")
			return
		}
		if r.URL.Path == "/v1/usage" && r.Method == "GET" {
			usage, err := s.Run.Usage(r.Context(), claims.AccountID)
			if err != nil {
				reject(w, 503, "unavailable")
				return
			}
			global, err := s.Run.Usage(r.Context(), "")
			if err != nil {
				reject(w, 503, "unavailable")
				return
			}
			active, queued := s.Queue.State()
			jsonReply(w, 200, map[string]any{"usage": usage, "project_run": global, "active": active, "queued": queued, "recording": s.Events != nil})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/requests/") {
			id := strings.TrimPrefix(r.URL.Path, "/v1/requests/")
			cancel := strings.HasSuffix(id, "/cancel")
			id = strings.TrimSuffix(id, "/cancel")
			if !budget.ValidID(id) || (!cancel && r.Method != "GET") || (cancel && r.Method != "POST") {
				reject(w, 400, "invalid_request")
				return
			}
			v, err := s.Run.Lookup(r.Context(), claims.AccountID, id)
			if err != nil {
				reject(w, 404, "not_found")
				return
			}
			if cancel {
				s.mu.Lock()
				if stop := s.active[id]; stop != nil {
					stop()
				}
				s.mu.Unlock()
			}
			jsonReply(w, 200, v)
			return
		}
		if r.URL.Path != "/v1/chat/completions" || r.Method != "POST" {
			reject(w, 404, "not_found")
			return
		}
		s.complete(w, r, workspace, token, claims)
	})
}
func (s *Server) complete(w http.ResponseWriter, r *http.Request, workspace, token string, claims capability.Claims) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBody))
	if err != nil {
		reject(w, 400, "invalid_request")
		return
	}
	body, names, err := Prepare(b, s.Run.Document.Allocation, s.Secret)
	if err != nil {
		reject(w, 400, "invalid_request")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	finished, err := s.admit(cancel)
	if err != nil {
		reject(w, 503, "run_draining")
		return
	}
	defer finished()
	release, err := s.Queue.Acquire(ctx, claims.AccountID)
	if err != nil {
		reject(w, 429, "capacity")
		return
	}
	defer release()
	// Authority is read again after waiting: neither a queued capability nor a
	// pre-queue grant snapshot authorizes a dispatch after disable/reset.
	current, err := s.authorize(ctx, token, workspace)
	if err != nil || current != claims {
		reject(w, 403, "stale_authorization")
		return
	}
	h := sha256.Sum256(body)
	id := r.Header.Get("X-Request-ID")
	if id == "" {
		id = UUID()
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		key = id
	}
	cost, err := s.Run.Document.Allocation.ReserveCost(int64(len(body)))
	if err != nil {
		reject(w, 400, "invalid_request")
		return
	}
	if s.Events != nil {
		if err = s.Events.Record(ctx, claims, id, "model.request", map[string]any{"request_sha256": hex.EncodeToString(h[:]), "model": budget.Model, "provider": budget.Provider, "profile": budget.Profile}); err != nil {
			reject(w, 503, "telemetry_unavailable")
			return
		}
	}
	reservation, exists, err := s.Run.Reserve(ctx, budget.Reservation{ID: id, Account: claims.AccountID, Key: key, Hash: hex.EncodeToString(h[:]), AuthVersion: claims.AuthVersion, Generation: claims.WorkspaceGeneration, Amount: cost})
	if err != nil {
		if errors.Is(err, budget.ErrExhausted) {
			reject(w, 429, "budget_exhausted")
		} else if errors.Is(err, budget.ErrConflict) {
			reject(w, 409, "conflict")
		} else {
			reject(w, 503, "unavailable")
		}
		return
	}
	w.Header().Set("X-Request-ID", reservation.ID)
	if exists {
		jsonReply(w, 409, map[string]any{"error": map[string]string{"code": "reconciliation_required", "message": "request already recorded; never replayed"}, "request": reservation})
		return
	}
	providerID := ""
	settled := false
	defer func() {
		if !settled {
			_ = s.Run.Unknown(id, providerID)
			if s.Events != nil {
				capture, cancelCapture := context.WithTimeout(context.Background(), 4*time.Second)
				defer cancelCapture()
				_ = s.Events.Record(capture, claims, id, "model.unknown", map[string]any{"reserved_usd_micros": cost})
			}
		}
	}()
	if err = s.Run.Dispatch(ctx, id); err != nil {
		reject(w, 503, "unavailable")
		return
	}
	s.mu.Lock()
	s.active[id] = cancel
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.active, id); s.mu.Unlock() }()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				check, cancelCheck := context.WithTimeout(ctx, 4*time.Second)
				_, err := s.authorize(check, token, workspace)
				cancelCheck()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	if s.Operations != nil {
		activity, cancelActivity := context.WithTimeout(ctx, 4*time.Second)
		err = s.Operations.Set(activity, workspace, id, true)
		cancelActivity()
		if err != nil {
			reject(w, 503, "workspace_unavailable")
			return
		}
		defer func() {
			activity, cancelActivity := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancelActivity()
			// Failure leaves the controller's bounded lease in place; it cannot
			// cause another provider dispatch or release a budget reservation.
			_ = s.Operations.Set(activity, workspace, id, false)
		}()
	}
	resp, err := s.Provider.Send(ctx, body)
	if err != nil {
		reject(w, 503, "provider_transport")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		code := "provider_transport"
		status := 503
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			code = "provider_authentication"
		}
		if resp.StatusCode == 429 {
			code = "provider_rate_limited"
			status = 429
		}
		reject(w, status, code)
		return
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		reject(w, 503, "provider_protocol")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		reject(w, 503, "unavailable")
		return
	}
	w.WriteHeader(200)
	result, err := Relay(resp.Body, w, flusher.Flush, names)
	providerID = result.ProviderID
	if err != nil {
		_, _ = io.WriteString(w, "data: {\"error\":{\"code\":\"provider_protocol\",\"message\":\"stream incomplete; reservation retained\"}}\n\n")
		flusher.Flush()
		return
	}
	if result.Cost != nil {
		if err = s.Run.Settle(id, providerID, *result.Cost); err != nil {
			_, _ = io.WriteString(w, "data: {\"error\":{\"code\":\"reconciliation_required\",\"message\":\"accounting unavailable\"}}\n\n")
			flusher.Flush()
			return
		}
		settled = true
	}
	if s.Events != nil {
		capture, cancelCapture := context.WithTimeout(context.Background(), 4*time.Second)
		err = s.Events.Record(capture, claims, id, "model.usage", map[string]any{"cost_usd_micros": result.Cost, "provider_id": providerID, "model": budget.Model, "provider": budget.Provider})
		cancelCapture()
		if err != nil {
			_, _ = io.WriteString(w, "data: {\"error\":{\"code\":\"telemetry_unavailable\",\"message\":\"capture incomplete\"}}\n\n")
			flusher.Flush()
			return
		}
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	flusher.Flush()
}
