package broker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/budget"
)

type StreamResult struct {
	ProviderID              string
	Cost                    *int64
	ModelSeen, ProviderSeen bool
}
type partialCall struct{ ID, Name, Arguments string }

var costPattern = regexp.MustCompile(`^[0-9]{1,16}(\.[0-9]{1,18})?([eE][+-]?[0-9]{1,2})?$`)

// DecimalCost rounds USD upward to integer microdollars without float parsing.
func DecimalCost(n json.Number) (int64, error) {
	if !costPattern.MatchString(n.String()) {
		return 0, budget.ErrInvalid
	}
	r, ok := new(big.Rat).SetString(n.String())
	if !ok || r.Sign() < 0 {
		return 0, budget.ErrInvalid
	}
	r.Mul(r, big.NewRat(1_000_000, 1))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if rem.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, budget.ErrInvalid
	}
	return q.Int64(), nil
}

// Relay parses whole UTF-8 SSE events while preserving deltas and call IDs.
// [DONE] is withheld until the entire bounded response and complete tool JSON
// are validated. A malformed/truncated stream cannot become an executable call.
func Relay(src io.Reader, dst io.Writer, flush func(), names map[string]bool) (StreamResult, error) {
	var result StreamResult
	limited := &io.LimitedReader{R: src, N: MaxBody + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), MaxBody)
	var data []string
	calls := map[int]partialCall{}
	done := false
	finish := ""
	frames := 0
	process := func() error {
		if len(data) == 0 {
			return nil
		}
		event := strings.Join(data, "\n")
		data = nil
		if done {
			return errors.New("data after completion")
		}
		if event == "[DONE]" {
			done = true
			return nil
		}
		if !utf8.ValidString(event) {
			return errors.New("invalid UTF-8")
		}
		var raw map[string]json.RawMessage
		if json.Unmarshal([]byte(event), &raw) != nil {
			return errors.New("invalid SSE JSON")
		}
		if _, ok := raw["error"]; ok {
			return errors.New("provider error")
		}
		var frame struct {
			ID       string `json:"id"`
			Model    string `json:"model"`
			Provider string `json:"provider"`
			Choices  []struct {
				Index        int     `json:"index"`
				FinishReason *string `json:"finish_reason"`
				Delta        struct {
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				Cost *json.Number `json:"cost"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(event), &frame) != nil {
			return errors.New("invalid frame")
		}
		if frame.Model != "" {
			if frame.Model != budget.Model {
				return errors.New("unapproved model")
			}
			result.ModelSeen = true
		}
		if frame.Provider != "" {
			if frame.Provider != "DeepInfra" && frame.Provider != budget.Provider {
				return errors.New("unapproved provider")
			}
			result.ProviderSeen = true
		}
		if frame.ID != "" {
			if len(frame.ID) > 256 || result.ProviderID != "" && result.ProviderID != frame.ID {
				return errors.New("inconsistent provider id")
			}
			result.ProviderID = frame.ID
		}
		if len(frame.Choices) > 1 {
			return errors.New("multiple choices")
		}
		for _, choice := range frame.Choices {
			if choice.Index != 0 {
				return errors.New("invalid choice")
			}
			if choice.FinishReason != nil {
				if *choice.FinishReason != "stop" && *choice.FinishReason != "tool_calls" {
					return errors.New("truncated completion")
				}
				finish = *choice.FinishReason
			}
			for _, delta := range choice.Delta.ToolCalls {
				if delta.Index < 0 || delta.Index >= 8 {
					return errors.New("too many calls")
				}
				p := calls[delta.Index]
				if delta.ID != "" {
					if p.ID != "" && p.ID != delta.ID {
						return errors.New("call id changed")
					}
					p.ID = delta.ID
				}
				if delta.Function.Name != "" {
					if p.Name != "" && p.Name != delta.Function.Name {
						return errors.New("call name changed")
					}
					p.Name = delta.Function.Name
				}
				p.Arguments += delta.Function.Arguments
				if len(p.Arguments) > 64<<10 {
					return errors.New("tool arguments too large")
				}
				calls[delta.Index] = p
			}
		}
		if frame.Usage != nil && frame.Usage.Cost != nil {
			cost, err := DecimalCost(*frame.Usage.Cost)
			if err != nil {
				return err
			}
			if result.Cost != nil && *result.Cost != cost {
				return errors.New("inconsistent provider cost")
			}
			result.Cost = &cost
		}
		if _, err := fmt.Fprintf(dst, "data: %s\n\n", event); err != nil {
			return err
		}
		flush()
		frames++
		return nil
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := process(); err != nil {
				return result, err
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		} else if !strings.HasPrefix(line, ":") {
			return result, errors.New("unsupported SSE field")
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if limited.N <= 0 {
		return result, errors.New("response too large")
	}
	if len(data) > 0 {
		return result, errors.New("unterminated SSE event")
	}
	if !done || frames == 0 || finish == "" || !result.ModelSeen || !result.ProviderSeen || result.ProviderID == "" {
		return result, errors.New("incomplete response or missing route identity")
	}
	ids := map[string]bool{}
	for _, p := range calls {
		if p.ID == "" || ids[p.ID] || !names[p.Name] || !json.Valid([]byte(p.Arguments)) || !bytes.HasPrefix(bytes.TrimSpace([]byte(p.Arguments)), []byte("{")) {
			return result, errors.New("invalid complete tool call")
		}
		ids[p.ID] = true
	}
	return result, nil
}
