//go:build e2e

// Package e2e drives the ask-gemini-mcp binary via JSON-RPC over
// stdio, simulating a real MCP client. These tests are excluded from
// `go test ./...`; run them with:
//
//	make test-e2e
//
// They make real Vertex AI calls, so set ASK_GEMINI_PROJECT and run
// `gcloud auth application-default login` first. ASK_GEMINI_TEST_BINARY
// must point at the built binary (defaults to ./dist/ask-gemini-mcp,
// which `make test-e2e` builds for you).
package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

// Harness drives a spawned ask-gemini-mcp process over stdio.
type Harness struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan []byte
	nextID atomic.Int64
}

// Start spawns the binary with the given flags, hooks up stdio, and
// registers cleanup so the binary is stopped at test end.
func Start(t *testing.T, args ...string) *Harness {
	return StartWithEnv(t, nil, args...)
}

// StartWithEnv is like Start but layers additional env vars on top of
// the test process's own environment. Use for per-test overrides such
// as ASK_GEMINI_REQUEST_TIMEOUT.
func StartWithEnv(t *testing.T, extraEnv map[string]string, args ...string) *Harness {
	t.Helper()

	binary := os.Getenv("ASK_GEMINI_TEST_BINARY")
	if binary == "" {
		t.Skip("ASK_GEMINI_TEST_BINARY not set; run via `make test-e2e`")
	}
	if os.Getenv("ASK_GEMINI_PROJECT") == "" && os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		t.Skip("ASK_GEMINI_PROJECT (or GOOGLE_CLOUD_PROJECT) not set; cannot reach Vertex AI")
	}

	cmd := exec.Command(binary, args...)
	if len(extraEnv) > 0 {
		cmd.Env = os.Environ()
		for k, v := range extraEnv {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	// Server logs go to stderr; surface them with `go test -v` so a
	// failing test shows the server's view of what went wrong.
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start binary: %v", err)
	}

	lines := make(chan []byte, 16)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			b := make([]byte, len(scanner.Bytes()))
			copy(b, scanner.Bytes())
			lines <- b
		}
	}()

	h := &Harness{t: t, cmd: cmd, stdin: stdin, lines: lines}
	t.Cleanup(h.Close)
	return h
}

// Close gracefully shuts the server down by closing stdin and waiting.
func (h *Harness) Close() {
	_ = h.stdin.Close()
	_ = h.cmd.Wait()
}

// Call sends a JSON-RPC request and waits up to timeout for the
// matching response.
func (h *Harness) Call(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	id := h.nextID.Add(1)
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	if err := json.NewEncoder(h.stdin).Encode(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	deadline := time.After(timeout)
	for {
		select {
		case line, ok := <-h.lines:
			if !ok {
				return nil, fmt.Errorf("server stdout closed before response (method=%s id=%d)", method, id)
			}
			var resp struct {
				ID     json.Number     `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(line, &resp); err != nil {
				return nil, fmt.Errorf("parse response: %w (line=%q)", err, string(line))
			}
			respID, err := resp.ID.Int64()
			if err != nil || respID != id {
				continue
			}
			if resp.Error != nil {
				return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			return resp.Result, nil
		case <-deadline:
			return nil, fmt.Errorf("timeout after %v waiting for response (method=%s id=%d)", timeout, method, id)
		}
	}
}

// CallTool is a convenience helper for tools/call: it returns the
// inner text from the first content block and the isError flag.
func (h *Harness) CallTool(name string, arguments any, timeout time.Duration) (string, bool, error) {
	res, err := h.Call("tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	}, timeout)
	if err != nil {
		return "", false, err
	}
	var wrap struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(res, &wrap); err != nil {
		return "", false, fmt.Errorf("parse tools/call result: %w", err)
	}
	if len(wrap.Content) == 0 {
		return "", wrap.IsError, fmt.Errorf("tools/call returned no content")
	}
	return wrap.Content[0].Text, wrap.IsError, nil
}
