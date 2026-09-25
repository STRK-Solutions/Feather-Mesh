package broker

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestGuidedTransportHistoryAndResponseAuthority(t *testing.T) {
	s, _ := setup(t)
	var r Request
	if err := json.Unmarshal(requestBytes(t), &r); err != nil {
		t.Fatal(err)
	}
	r.Tools = []WireTool{}
	r.ToolChoice = "none"
	call := Call{ID: "ordinary-call", Type: "function"}
	call.Function.Name = "help__lookup"
	call.Function.Arguments = `{"topic":"resolve"}`
	result := `{"topic":"resolve","path":"/private/hidden"}`
	r.Messages = append(r.Messages, Message{Role: "assistant", ToolCalls: []Call{call}}, Message{Role: "tool", ToolCallID: call.ID, Content: &result})
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	out, names, err := Prepare(encoded, s.Run.Document.Allocation, "")
	if err != nil {
		t.Fatal(err)
	}
	var prepared Request
	if err := json.Unmarshal(out, &prepared); err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 || len(prepared.Tools) != 0 || prepared.ToolChoice != "none" || !bytes.Contains(out, []byte(`"tools":[]`)) {
		t.Fatal("guided request gained tool authority", string(out))
	}
	if prepared.Messages[1].ToolCalls[0].Function.Name != "help__lookup" || prepared.Messages[2].ToolCallID != call.ID || strings.Contains(string(out), "/private") {
		t.Fatal("history correlation or disclosure changed", string(out))
	}
	for _, tc := range []struct {
		name, delta, finish string
		allowed             bool
	}{
		{"text", `{"content":"Choose a version."}`, "stop", true},
		{"new_tool", `{"tool_calls":[{"index":0,"id":"new-call","type":"function","function":{"name":"help__lookup","arguments":"{}"}}]}`, "tool_calls", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := "data: {\"id\":\"guided-response\",\"model\":\"deepseek/deepseek-v4.1-flash\",\"provider\":\"DeepInfra\",\"choices\":[{\"index\":0,\"delta\":" + tc.delta + ",\"finish_reason\":\"" + tc.finish + "\"}]}\n\ndata: [DONE]\n\n"
			var dst bytes.Buffer
			_, err := Relay(strings.NewReader(stream), &dst, func() {}, names)
			if (err == nil) != tc.allowed || (!tc.allowed && strings.Contains(dst.String(), "[DONE]")) {
				t.Fatal("incorrect guided response authority", err, dst.String())
			}
		})
	}

	r.Messages[1].ToolCalls[0].Function.Name = "shell__execute"
	encoded, _ = json.Marshal(r)
	if _, _, err := Prepare(encoded, s.Run.Document.Allocation, ""); err == nil {
		t.Fatal("unapproved historical tool accepted")
	}
}

func TestGuidedTransportRequiresExplicitToolChoice(t *testing.T) {
	s, _ := setup(t)
	for _, tc := range []struct {
		choice string
		tools  []WireTool
		valid  bool
	}{
		{"none", nil, true},
		{"none", []WireTool{}, true},
		{"auto", nil, false},
		{"", nil, false},
		{"required", nil, false},
		{"none", ApprovedTools(), false},
		{"auto", ApprovedTools(), true},
	} {
		var r Request
		_ = json.Unmarshal(requestBytes(t), &r)
		r.Tools, r.ToolChoice = tc.tools, tc.choice
		encoded, _ := json.Marshal(r)
		_, _, err := Prepare(encoded, s.Run.Document.Allocation, "")
		if (err == nil) != tc.valid {
			t.Fatalf("choice=%q tools=%d: %v", tc.choice, len(tc.tools), err)
		}
	}
}
