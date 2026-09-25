// Package collector stores only bounded, redacted structured events. Research
// identity mapping and operational billing stay outside its event database.
package collector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"regexp"
	"strings"
	"time"
)

const Protocol = "feam.web.event.v1"
const MaxEvent = 64 << 10
const MaxBatch = 8 << 20

var ErrRejected = errors.New("event rejected")
var ErrBackpressure = errors.New("capture unavailable; session must pause or declare unrecorded")
var digestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
var labelRE = regexp.MustCompile(`^[a-zA-Z0-9_.:/-]{1,128}$`)

type Event struct {
	Protocol       string  `json:"protocol"`
	EventID        string  `json:"event_id"`
	StreamID       string  `json:"stream_id"`
	Sequence       int64   `json:"sequence"`
	DeploymentID   string  `json:"deployment_id"`
	WorkspaceID    string  `json:"workspace_id"`
	Generation     int64   `json:"generation"`
	ParticipantID  string  `json:"participant_id"`
	ConversationID string  `json:"conversation_id"`
	RequestID      string  `json:"request_id"`
	ProposalID     *string `json:"proposal_id"`
	OccurredAt     string  `json:"occurred_at"`
	Kind           string  `json:"kind"`
	Trust          string  `json:"trust"`
	SoftwareSHA256 string  `json:"software_sha256"`
	ProfileSHA256  string  `json:"profile_sha256"`
	DatasetSHA256  string  `json:"dataset_sha256"`
	PayloadSHA256  string  `json:"payload_sha256"`
	Payload        Payload `json:"payload"`
	Synthetic      bool    `json:"synthetic"`
}
type Payload struct {
	Text              string `json:"text,omitempty"`
	Tool              string `json:"tool,omitempty"`
	Decision          string `json:"decision,omitempty"`
	Outcome           string `json:"outcome,omitempty"`
	ReceiptSHA256     string `json:"receipt_sha256,omitempty"`
	RequestSHA256     string `json:"request_sha256,omitempty"`
	ErrorCode         string `json:"error_code,omitempty"`
	Model             string `json:"model,omitempty"`
	Provider          string `json:"provider,omitempty"`
	ProviderID        string `json:"provider_id,omitempty"`
	CostUSDMicros     *int64 `json:"cost_usd_micros,omitempty"`
	ReservedUSDMicros int64  `json:"reserved_usd_micros,omitempty"`
	UnknownCost       bool   `json:"unknown_cost,omitempty"`
	InputTokens       int64  `json:"input_tokens,omitempty"`
	OutputTokens      int64  `json:"output_tokens,omitempty"`
	LatencyMS         int64  `json:"latency_ms,omitempty"`
	Rating            int    `json:"rating,omitempty"`
	TaskTemplate      string `json:"task_template,omitempty"`
	DatasetFamily     string `json:"dataset_family,omitempty"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func (e Event) Validate() error {
	if e.Protocol != Protocol || e.Sequence < 1 || e.Generation < 1 {
		return ErrRejected
	}
	for _, id := range []string{e.EventID, e.StreamID, e.DeploymentID, e.WorkspaceID, e.ParticipantID, e.ConversationID, e.RequestID} {
		if !control.ValidID(id) {
			return ErrRejected
		}
	}
	if e.ProposalID != nil && !control.ValidID(*e.ProposalID) {
		return ErrRejected
	}
	for _, s := range []string{e.SoftwareSHA256, e.ProfileSHA256, e.DatasetSHA256} {
		if !digestRE.MatchString(s) {
			return ErrRejected
		}
	}
	when, err := time.Parse(time.RFC3339Nano, e.OccurredAt)
	if err != nil || !strings.HasSuffix(e.OccurredAt, "Z") || when.After(time.Now().Add(time.Minute)) {
		return ErrRejected
	}
	switch e.Kind {
	case "request", "proposal", "review", "outcome", "usage", "feedback", "gap", "withdrawal":
	default:
		return ErrRejected
	}
	switch e.Trust {
	case "client_reported", "broker_observed", "independently_verified":
	default:
		return ErrRejected
	}
	if e.Trust == "independently_verified" && e.Kind == "outcome" && !digestRE.MatchString(e.Payload.ReceiptSHA256) {
		return ErrRejected
	}
	if e.Payload.CostUSDMicros != nil && *e.Payload.CostUSDMicros < 0 || e.Payload.ReservedUSDMicros < 0 || e.Payload.InputTokens < 0 || e.Payload.OutputTokens < 0 || e.Payload.LatencyMS < 0 || e.Payload.Rating < -1 || e.Payload.Rating > 1 {
		return ErrRejected
	}
	for _, s := range []string{e.Payload.Tool, e.Payload.Decision, e.Payload.Outcome, e.Payload.ErrorCode, e.Payload.Model, e.Payload.Provider, e.Payload.TaskTemplate, e.Payload.DatasetFamily} {
		if s != "" && (!labelRE.MatchString(s) || Redact(s) != s) {
			return ErrRejected
		}
	}
	for _, s := range []string{e.Payload.ReceiptSHA256, e.Payload.RequestSHA256} {
		if s != "" && !digestRE.MatchString(s) {
			return ErrRejected
		}
	}
	b, _ := json.Marshal(e)
	if len(b) > MaxEvent {
		return ErrRejected
	}
	return nil
}

var emails = regexp.MustCompile(`(?i)[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9.-]+\.[a-z]{2,}`)
var credentials = regexp.MustCompile(`(?i)(bearer\s+[^\s]+|(?:sk-|sk_)[a-z0-9_-]{8,}|(?:token|password|api[_-]?key|authorization)\s*[:=]\s*[^\s,;]+)`)
var privatePaths = regexp.MustCompile(`(?:/Users/|/home/|/private/|/var/|/run/|[A-Za-z]:\\)[^\s"'<>]+`)

func Redact(s string) string {
	if len(s) > 4096 {
		s = s[:4096]
	}
	s = credentials.ReplaceAllString(s, "[credential]")
	s = emails.ReplaceAllString(s, "[email]")
	s = privatePaths.ReplaceAllString(s, "[private-path]")
	return s
}
func (e *Event) Sanitize(allowText bool) {
	if !allowText && e.Payload.Text != "" {
		e.Payload.Text = "[content omitted by research policy]"
	} else {
		e.Payload.Text = Redact(e.Payload.Text)
		if strings.Count(e.Payload.Text, "\n") > 4 || strings.Contains(e.Payload.Text, "data:") || strings.Contains(e.Payload.Text, "BEGIN PRIVATE") {
			e.Payload.Text = "[structured or sensitive content omitted]"
		}
	}
	if e.Payload.ProviderID != "" {
		e.Payload.ProviderID = "sha256:" + digest([]byte(e.Payload.ProviderID))
	}
	if len(e.Payload.ProviderID) > 128 {
		e.Payload.ProviderID = "[provider ID omitted]"
	}
	b, _ := json.Marshal(e.Payload)
	e.PayloadSHA256 = digest(b)
}

type Participant struct {
	AccountID        string `json:"account_id"`
	ParticipantID    string `json:"participant_id"`
	ConsentReference string `json:"consent_reference"`
	Eligible         bool   `json:"eligible"`
	AllowText        bool   `json:"allow_text"`
	Synthetic        bool   `json:"synthetic"`
}
