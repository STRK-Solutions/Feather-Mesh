package broker

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
)

const MaxBody = 1 << 20
const MaxContext = 32000

//go:embed tools.json
var toolBytes []byte

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}
type WireTool struct {
	Type     string `json:"type"`
	Function Tool   `json:"function"`
}
type Call struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
type Message struct {
	Role       string  `json:"role"`
	Content    *string `json:"content"`
	ToolCallID string  `json:"tool_call_id,omitempty"`
	ToolCalls  []Call  `json:"tool_calls,omitempty"`
}
type Request struct {
	Model       string      `json:"model"`
	MaxTokens   int64       `json:"max_tokens"`
	Temperature json.Number `json:"temperature"`
	Reasoning   struct {
		Enabled bool `json:"enabled"`
	} `json:"reasoning"`
	Messages      []Message  `json:"messages"`
	Tools         []WireTool `json:"tools"`
	ToolChoice    string     `json:"tool_choice"`
	Stream        bool       `json:"stream"`
	StreamOptions struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
	Provider json.RawMessage `json:"provider"`
}

func StrictDecode(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	d.UseNumber()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("invalid trailing JSON")
	}
	return nil
}
func ApprovedTools() []WireTool {
	var tools []Tool
	if StrictDecode(toolBytes, &tools) != nil {
		panic("invalid embedded schema")
	}
	out := make([]WireTool, len(tools))
	for i, t := range tools {
		t.Name = strings.ReplaceAll(t.Name, ".", "__")
		out[i] = WireTool{"function", t}
	}
	return out
}
func Redact(s, secret string) string {
	if secret != "" {
		s = strings.ReplaceAll(s, secret, "[credential]")
	}
	parts := strings.Fields(s)
	for i, p := range parts {
		v := strings.TrimLeft(p, `"'([{`)
		if (strings.Contains(v, "/") && !strings.HasPrefix(v, "product://") && !strings.HasPrefix(v, "feam.")) || strings.HasPrefix(v, "~") || strings.Contains(v, ":\\") || strings.Contains(v, "@") || strings.HasPrefix(v, "sk-") || strings.Contains(v, "Bearer") || strings.Contains(v, "API_KEY=") {
			parts[i] = "[redacted]"
		} else {
			parts[i] = strings.Map(func(r rune) rune {
				if unicode.IsControl(r) {
					return -1
				}
				return r
			}, p)
		}
	}
	return strings.Join(parts, " ")
}

var metadataFields = func() map[string]bool {
	m := map[string]bool{}
	for _, s := range strings.Fields("operations drafts summary overwrite question choices protocol tool status result error kind operation_id handle reference version manifest_revision expected_manifest_revision data_kind data_format quality classification entries coverage namespace available revision observed_at freshness integrity_verified asset_count estimated_asset_bytes assets id role media_type size next_cursor snapshot_fingerprint result_limit truncated topic excerpt state completed_at input_tokens output_tokens cost_usd missing_fields title description tags") {
		m[s] = true
	}
	return m
}()

func scrub(v any, secret string, metadata bool) any {
	switch x := v.(type) {
	case string:
		return Redact(x, secret)
	case []any:
		if len(x) > 100 {
			x = x[:100]
		}
		for i := range x {
			x[i] = scrub(x[i], secret, metadata)
		}
		return x
	case map[string]any:
		y := map[string]any{}
		for k, v := range x {
			if !metadata || metadataFields[k] {
				y[k] = scrub(v, secret, metadata)
			}
		}
		return y
	default:
		return v
	}
}

// Prepare accepts the existing RouterProvider wire protocol and replaces all
// routing policy with signed operator policy. Client schema/profile changes
// cannot introduce a tool, model, provider, URL, reasoning, or retry option.
func Prepare(b []byte, a budget.Allocation, secret string) ([]byte, map[string]bool, error) {
	var r Request
	if len(b) > MaxBody || StrictDecode(b, &r) != nil || r.Model != budget.Model || !r.Stream || !r.StreamOptions.IncludeUsage || r.Reasoning.Enabled || r.ToolChoice != "auto" || r.MaxTokens < 1 || r.MaxTokens > a.MaxOutputTokens || r.Temperature.String() != "0" || len(r.Messages) == 0 || len(r.Messages) > 100 || len(r.Tools) == 0 || len(r.Tools) > 11 {
		return nil, nil, budget.ErrInvalid
	}
	approved := map[string]WireTool{}
	for _, t := range ApprovedTools() {
		approved[t.Function.Name] = t
	}
	names := map[string]bool{}
	for i, t := range r.Tools {
		expected, ok := approved[t.Function.Name]
		if !ok || names[t.Function.Name] || !reflect.DeepEqual(t, expected) {
			return nil, nil, budget.ErrInvalid
		}
		names[t.Function.Name] = true
		r.Tools[i] = expected
	}
	total := 0
	for i := range r.Messages {
		m := &r.Messages[i]
		if m.Role != "system" && m.Role != "assistant" && m.Role != "tool" && m.Role != "user" {
			return nil, nil, budget.ErrInvalid
		}
		if m.Content != nil {
			total += len(*m.Content)
			text := Redact(*m.Content, secret)
			if m.Role == "tool" {
				var v any
				if json.Unmarshal([]byte(*m.Content), &v) != nil {
					return nil, nil, budget.ErrInvalid
				}
				filtered, _ := json.Marshal(scrub(v, secret, true))
				text = string(filtered)
			}
			m.Content = &text
		}
		if len(m.ToolCalls) > 8 {
			return nil, nil, budget.ErrInvalid
		}
		for j := range m.ToolCalls {
			c := &m.ToolCalls[j]
			if c.ID == "" || len(c.ID) > 128 || c.Type != "function" || !names[c.Function.Name] {
				return nil, nil, budget.ErrInvalid
			}
			var args any
			if json.Unmarshal([]byte(c.Function.Arguments), &args) != nil {
				return nil, nil, budget.ErrInvalid
			}
			v, _ := json.Marshal(scrub(args, secret, false))
			c.Function.Arguments = string(v)
		}
	}
	if total > MaxContext {
		return nil, nil, budget.ErrInvalid
	}
	// Provider fields supplied by the sandbox are never forwarded.
	provider := map[string]any{"only": []string{budget.Provider}, "allow_fallbacks": false, "require_parameters": true, "data_collection": "deny", "max_price": map[string]json.Number{"prompt": usd(a.InputPrice), "completion": usd(a.OutputPrice)}}
	r.Provider, _ = json.Marshal(provider)
	r.MaxTokens = a.MaxOutputTokens
	out, err := json.Marshal(r)
	if err != nil || len(out) > MaxBody {
		return nil, nil, budget.ErrInvalid
	}
	return out, names, nil
}
func usd(micros int64) json.Number {
	return json.Number(fmt.Sprintf("%d.%06d", micros/1_000_000, micros%1_000_000))
}
