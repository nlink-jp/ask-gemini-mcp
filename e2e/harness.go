//go:build e2e

// Package e2e drives the ask-gemini-mcp binary over stdio with
// nlink-jp/mcp's client, as a real MCP client does. These tests are
// excluded from `go test ./...`; run them with:
//
//	make test-e2e
//
// They make real Vertex AI calls, so set ASK_GEMINI_PROJECT and run
// `gcloud auth application-default login` first. ASK_GEMINI_TEST_BINARY
// must point at the built binary (defaults to ./dist/ask-gemini-mcp,
// which `make test-e2e` builds for you).
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/nlink-jp/mcp"
	"github.com/nlink-jp/mcp/client"
)

// Harness drives a spawned ask-gemini-mcp process over stdio.
type Harness struct {
	t     *testing.T
	cmd   *exec.Cmd
	stdin io.WriteCloser
	cl    *client.Client
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
	// Skip only when neither env nor config file can provide a project
	// ID — running the binary in that case would error out at startup
	// and the test failure would obscure the real cause.
	if !hasProjectConfigured() {
		t.Skip("no GCP project configured: set ASK_GEMINI_PROJECT, GOOGLE_CLOUD_PROJECT, or create ~/.config/ask-gemini-mcp/config.toml")
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

	// The binary is opened the way a dual-era MCP client opens it, with
	// nlink-jp/mcp's client (organization ADR-030 §6): nothing here
	// writes the handshake by hand.
	conn, err := client.NewLineConn(stdout, stdin, func() { _ = cmd.Process.Kill() }, client.LineOptions{})
	if err != nil {
		t.Fatalf("line conn: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cl, err := client.Open(ctx, conn, client.Options{Client: mcp.Implementation{Name: "e2e", Version: "0"}})
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("open: %v", err)
	}

	h := &Harness{t: t, cmd: cmd, stdin: stdin, cl: cl}
	t.Cleanup(h.Close)
	return h
}

// hasProjectConfigured reports whether the binary will be able to
// resolve a project at startup (either env or config file).
func hasProjectConfigured() bool {
	if os.Getenv("ASK_GEMINI_PROJECT") != "" || os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".config", "ask-gemini-mcp", "config.toml"))
	return err == nil
}

// Close gracefully shuts the server down by closing stdin and waiting.
func (h *Harness) Close() {
	_ = h.cl.Close()
	_ = h.stdin.Close()
	_ = h.cmd.Wait()
}

// Client is the opened MCP client, for what CallTool does not cover.
func (h *Harness) Client() *client.Client { return h.cl }

// CallTool calls one tool: it returns the inner text from the first
// content block and the isError flag. A protocol error is err.
func (h *Harness) CallTool(name string, arguments any, timeout time.Duration) (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	res, err := h.cl.CallTool(ctx, name, arguments, nil)
	if err != nil {
		return "", false, err
	}
	var block struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if len(res.Content) == 0 {
		return "", res.IsError, fmt.Errorf("tools/call returned no content")
	}
	if err := json.Unmarshal(res.Content[0], &block); err != nil {
		return "", res.IsError, fmt.Errorf("parse content block: %w", err)
	}
	return block.Text, res.IsError, nil
}
