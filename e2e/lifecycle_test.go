//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLifecycle_InitializeListCall(t *testing.T) {
	h := Start(t)

	// 1. initialize
	res, err := h.Call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
	}, 10*time.Second)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	var initRes struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(res, &initRes); err != nil {
		t.Fatalf("parse initialize result: %v", err)
	}
	if initRes.ProtocolVersion != "2024-11-05" {
		t.Errorf("protocolVersion = %q, want 2024-11-05", initRes.ProtocolVersion)
	}
	if initRes.ServerInfo.Name != "ask-gemini-mcp" {
		t.Errorf("serverInfo.name = %q, want ask-gemini-mcp", initRes.ServerInfo.Name)
	}

	// 2. tools/list — must include ask_gemini.
	res, err = h.Call("tools/list", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var listRes struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(res, &listRes); err != nil {
		t.Fatalf("parse tools/list: %v", err)
	}
	var foundAsk bool
	for _, tool := range listRes.Tools {
		if tool.Name == "ask_gemini" {
			foundAsk = true
		}
	}
	if !foundAsk {
		t.Fatalf("ask_gemini missing from tools/list: %v", listRes.Tools)
	}

	// 3. tools/call — ask Gemini something with a deterministic-ish
	// answer. We accept any non-empty reply; the goal is to prove the
	// wire path end-to-end, not to test Gemini's accuracy.
	text, isErr, err := h.CallTool("ask_gemini", map[string]any{
		"prompt": "Reply with exactly the word: pong",
	}, 60*time.Second)
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if isErr {
		t.Fatalf("isError on success path; payload: %s", text)
	}
	if strings.TrimSpace(text) == "" {
		t.Fatalf("empty reply from Gemini")
	}
}
