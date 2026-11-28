//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/mcp"
)

func TestLifecycle_OpenListCall(t *testing.T) {
	h := Start(t)

	// 1. open — nlink-jp/mcp's client lands in the modern era — and
	// tools/list, which must include ask_gemini.
	cl := h.Client()
	if cl.Era() != mcp.EraModern {
		t.Errorf("era = %v, want the modern era", cl.Era())
	}
	if cl.Server().Name != "ask-gemini-mcp" {
		t.Errorf("serverInfo.name = %q, want ask-gemini-mcp", cl.Server().Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, err := cl.ListTools(ctx)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var foundAsk bool
	for _, tool := range list.Tools {
		if tool.Name == "ask_gemini" {
			foundAsk = true
		}
	}
	if !foundAsk {
		t.Fatalf("ask_gemini missing from tools/list: %v", list.Tools)
	}

	// 2. tools/call — ask Gemini something with a deterministic-ish
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
