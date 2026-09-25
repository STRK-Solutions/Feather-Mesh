package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/capability"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/ipc"
	"io"
	"net/http"
	"strings"
	"time"
)

type Authority interface {
	Check(context.Context, capability.Claims) error
}
type ControlAuthority struct{ Client *http.Client }

func (a ControlAuthority) Check(ctx context.Context, c capability.Claims) error {
	b, _ := json.Marshal(control.Check{AccountID: c.AccountID, AuthVersion: c.AuthVersion, WorkspaceID: c.WorkspaceID, WorkspaceGeneration: c.WorkspaceGeneration, GrantVersion: c.GrantVersion, DeploymentID: c.DeploymentID, ActivationGeneration: c.ActivationGeneration})
	r, e := http.NewRequestWithContext(ctx, "POST", "http://control/v1/authorize", bytes.NewReader(b))
	if e != nil {
		return e
	}
	resp, e := a.Client.Do(r)
	if e != nil {
		return ErrRejected
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ErrRejected
	}
	return nil
}

type Server struct {
	Store                                 *Store
	Capabilities                          *capability.Store
	Authority                             Authority
	Participants                          map[string]Participant
	Archive                               Archive
	BrokerUID, ControllerUID, VerifierUID uint32
	ReviewerUIDs                          []uint32
	Reviewer                              string
}

func (s *Server) participant(c capability.Claims) (Participant, bool) {
	p, ok := s.Participants[c.AccountID]
	return p, ok && p.Eligible && p.ConsentReference != "" && control.ValidID(p.ParticipantID)
}
func (s *Server) Workspace(workspace string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/events" || r.URL.RawQuery != "" {
			http.Error(w, "submission only", 404)
			return
		}
		if len(r.Header.Values("Authorization")) != 1 {
			http.Error(w, "unauthenticated", 401)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims, e := s.Capabilities.Check(r.Context(), token, workspace, "event")
		if e != nil || s.Authority.Check(r.Context(), claims) != nil {
			http.Error(w, "forbidden", 403)
			return
		}
		p, ok := s.participant(claims)
		if !ok {
			http.Error(w, "capture not eligible", 403)
			return
		}
		var event Event
		if ipc.Decode(w, r, &event) != nil || event.Trust != "client_reported" || event.ParticipantID != p.ParticipantID || event.WorkspaceID != workspace || event.Generation != claims.WorkspaceGeneration || event.DeploymentID != claims.DeploymentID {
			http.Error(w, "invalid event", 400)
			return
		}
		event.Synthetic = p.Synthetic
		event.Sanitize(p.AllowText)
		ack, e := s.Store.Append(r.Context(), event)
		if e != nil {
			status := 400
			if e == ErrBackpressure {
				status = 503
			}
			http.Error(w, e.Error(), status)
			return
		}
		reply(w, ack)
	})
}
func (s *Server) Control() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /v1/capabilities/mint", ipc.RequireUID([]uint32{s.ControllerUID}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c capability.Claims
		if ipc.Decode(w, r, &c) != nil || c.Operation != "event" || !c.Valid(time.Now()) || s.Authority.Check(r.Context(), c) != nil {
			http.Error(w, "forbidden", 403)
			return
		}
		if _, ok := s.participant(c); !ok {
			http.Error(w, "ineligible", 403)
			return
		}
		token, e := s.Capabilities.Mint(r.Context(), c)
		if e != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		reply(w, map[string]string{"capability": token})
	})))
	mux.Handle("POST /v1/capabilities/revoke", ipc.RequireUID([]uint32{s.ControllerUID}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			WorkspaceID string `json:"workspace_id"`
		}
		if ipc.Decode(w, r, &in) != nil || s.Capabilities.Revoke(r.Context(), in.WorkspaceID) != nil {
			http.Error(w, "revoke unavailable", 503)
			return
		}
		reply(w, map[string]string{"status": "revoked"})
	})))
	mux.Handle("POST /v1/trusted-events", ipc.RequireUID([]uint32{s.BrokerUID}, http.HandlerFunc(s.trusted)))
	mux.Handle("POST /v1/verified-events", ipc.RequireUID([]uint32{s.VerifierUID}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Claims capability.Claims `json:"claims"`
			Event  Event             `json:"event"`
		}
		if ipc.Decode(w, r, &in) != nil || s.Authority.Check(r.Context(), in.Claims) != nil {
			http.Error(w, "forbidden", 403)
			return
		}
		p, ok := s.participant(in.Claims)
		if !ok || in.Event.ParticipantID != p.ParticipantID || in.Event.WorkspaceID != in.Claims.WorkspaceID || in.Event.Generation != in.Claims.WorkspaceGeneration || in.Event.DeploymentID != in.Claims.DeploymentID || in.Event.Trust != "independently_verified" {
			http.Error(w, "invalid verification", 400)
			return
		}
		in.Event.Synthetic = p.Synthetic
		in.Event.Sanitize(p.AllowText)
		ack, e := s.Store.Append(r.Context(), in.Event)
		if e != nil {
			http.Error(w, "capture unavailable", 503)
			return
		}
		reply(w, ack)
	})))
	mux.Handle("POST /v1/research/withdraw", ipc.RequireUID(s.ReviewerUIDs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ParticipantID string `json:"participant_id"`
		}
		if ipc.Decode(w, r, &in) != nil {
			http.Error(w, "invalid withdrawal", 400)
			return
		}
		if e := s.Store.Withdraw(r.Context(), s.Archive, in.ParticipantID, "withdrawal"); e != nil {
			http.Error(w, "withdrawal committed; archive reconciliation required", 503)
			return
		}
		reply(w, map[string]string{"status": "verified"})
	})))
	mux.Handle("POST /v1/research/export", ipc.RequireUID(s.ReviewerUIDs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ParticipantID string   `json:"participant_id"`
			Split         string   `json:"split"`
			Reviews       []Review `json:"reviews"`
		}
		if ipc.Decode(w, r, &in) != nil {
			http.Error(w, "invalid export", 400)
			return
		}
		out, e := s.Store.Export(r.Context(), s.Archive, s.Reviewer, in.ParticipantID, in.Split, in.Reviews)
		if e != nil {
			http.Error(w, "export rejected or archive pending", 409)
			return
		}
		reply(w, out)
	})))
	mux.Handle("POST /v1/archive/flush", ipc.RequireUID(s.ReviewerUIDs, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e := s.Store.Flush(r.Context(), s.Archive); e != nil {
			http.Error(w, "archive pending", 503)
			return
		}
		if e := s.Store.ReconcileDeletions(r.Context(), s.Archive); e != nil {
			http.Error(w, "deletion reconciliation pending", 503)
			return
		}
		if e := s.Store.TeardownReady(r.Context()); e != nil {
			http.Error(w, "teardown blocked", 409)
			return
		}
		reply(w, map[string]string{"status": "verified"})
	})))
	return mux
}
func (s *Server) trusted(w http.ResponseWriter, r *http.Request) {
	var in TrustedRecord
	if ipc.Decode(w, r, &in) != nil || !control.ValidID(in.RequestID) || s.Authority.Check(r.Context(), in.Claims) != nil {
		http.Error(w, "invalid trusted event", 400)
		return
	}
	p, ok := s.participant(in.Claims)
	if !ok {
		http.Error(w, "capture ineligible", 403)
		return
	}
	var raw struct {
		RequestSHA256     string `json:"request_sha256"`
		Model             string `json:"model"`
		Provider          string `json:"provider"`
		Profile           string `json:"profile"`
		CostUSDMicros     *int64 `json:"cost_usd_micros"`
		ProviderID        string `json:"provider_id"`
		ReservedUSDMicros int64  `json:"reserved_usd_micros"`
	}
	decoder := json.NewDecoder(bytes.NewReader(in.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&raw) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid payload", 400)
		return
	}
	seq := int64(1)
	kind := "request"
	switch in.Kind {
	case "model.request":
	case "model.usage":
		seq = 2
		kind = "usage"
	case "model.unknown":
		seq = 3
		kind = "gap"
	default:
		http.Error(w, "invalid kind", 400)
		return
	}
	event := Event{Protocol: Protocol, EventID: stableID(in.RequestID + in.Kind), StreamID: in.RequestID, Sequence: seq, DeploymentID: in.Claims.DeploymentID, WorkspaceID: in.Claims.WorkspaceID, Generation: in.Claims.WorkspaceGeneration, ParticipantID: p.ParticipantID, ConversationID: in.RequestID, RequestID: in.RequestID, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Kind: kind, Trust: "broker_observed", SoftwareSHA256: in.SoftwareSHA256, ProfileSHA256: in.ProfileSHA256, DatasetSHA256: in.DatasetSHA256, Payload: Payload{RequestSHA256: raw.RequestSHA256, Model: raw.Model, Provider: raw.Provider, ProviderID: raw.ProviderID, CostUSDMicros: raw.CostUSDMicros, ReservedUSDMicros: raw.ReservedUSDMicros, UnknownCost: in.Kind == "model.unknown"}}
	var previous string
	if s.Store.DB.QueryRowContext(r.Context(), `SELECT body FROM events WHERE id=?`, event.EventID).Scan(&previous) == nil {
		var old Event
		if json.Unmarshal([]byte(previous), &old) == nil {
			event.OccurredAt = old.OccurredAt
		}
	}
	event.Synthetic = p.Synthetic
	event.Sanitize(false)
	ack, e := s.Store.Append(r.Context(), event)
	if e != nil {
		http.Error(w, "capture unavailable", 503)
		return
	}
	reply(w, ack)
}
func stableID(s string) string {
	h := digest([]byte(s))
	return fmt.Sprintf("%s-%s-4%s-a%s-%s", h[:8], h[8:12], h[13:16], h[17:20], h[20:32])
}
func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
