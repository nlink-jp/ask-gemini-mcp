package cmd

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/nlink-jp/mcp"
	"github.com/nlink-jp/mcp/client"

	"github.com/nlink-jp/ask-gemini-mcp/internal/config"
	"github.com/nlink-jp/ask-gemini-mcp/internal/tools"
)

// withConfig points the server at a config file of its own and clears the
// variables that would override it, so the developer's settings never reach
// the test.
func withConfig(t *testing.T) {
	t.Helper()
	for _, v := range []string{"ASK_GEMINI_PROJECT", "GOOGLE_CLOUD_PROJECT", "ASK_GEMINI_LOCATION", "GOOGLE_CLOUD_LOCATION", "ASK_GEMINI_MODEL", "ASK_GEMINI_REQUEST_TIMEOUT", "ASK_GEMINI_LOG_LEVEL"} {
		t.Setenv(v, "")
	}
	cfgFile := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfgFile, []byte("[gcp]\nproject = \"test-project\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	orig := flagConfig
	flagConfig = cfgFile
	t.Cleanup(func() { flagConfig = orig })
	// The Gemini client is not built: a test of the wiring needs no
	// Google credentials and asks nothing.
	origAsker := newAsker
	newAsker = func(context.Context, *config.Config, *slog.Logger) (tools.Asker, error) { return nopAsker{}, nil }
	t.Cleanup(func() { newAsker = origAsker })

}

// withStreams hands serve the given stdin and stdout.
func withStreams(t *testing.T, in io.Reader, out io.Writer) {
	t.Helper()
	origIn, origOut := serveStdin, serveStdout
	serveStdin, serveStdout = in, out
	t.Cleanup(func() { serveStdin, serveStdout = origIn, origOut })
}

// SIGINT and SIGTERM end the server through its ctx, even while stdin is
// open (server-migration §2), and both are the ones registered.
func TestASignalEndsTheServer(t *testing.T) {
	withConfig(t)
	orig := notifyContext
	t.Cleanup(func() { notifyContext = orig })
	var registered []os.Signal
	notifyContext = func(parent context.Context, sigs ...os.Signal) (context.Context, context.CancelFunc) {
		registered = sigs
		ctx, cancel := context.WithCancel(parent)
		go func() { time.Sleep(100 * time.Millisecond); cancel() }() // the signal arrives
		return ctx, cancel
	}
	inR, inW := io.Pipe()
	defer func() { _ = inW.Close() }()
	withStreams(t, inR, io.Discard)
	done := make(chan error, 1)
	go func() { done <- run(nil, nil) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not end on the signal")
	}
	for _, want := range []os.Signal{syscall.SIGINT, syscall.SIGTERM} {
		if !slices.Contains(registered, want) {
			t.Errorf("%v is not among the signals that end the server: %v", want, registered)
		}
	}
}

// What serve runs opens with nlink-jp/mcp's client in the modern era, lists
// its one tool, advertises no instructions, and returns nil once stdin ends.
func TestServeOpensWithTheModuleClient(t *testing.T) {
	withConfig(t)
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	withStreams(t, inR, outW)
	done := make(chan error, 1)
	go func() {
		done <- run(nil, nil)
		_ = outW.Close()
	}()
	conn, err := client.NewLineConn(outR, inW, func() { _ = inW.Close() }, client.LineOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cl, err := client.Open(ctx, conn, client.Options{Client: mcp.Implementation{Name: "test", Version: "0"}})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if cl.Era() != mcp.EraModern || cl.Server().Name != "ask-gemini-mcp" || cl.Instructions() != "" {
		t.Errorf("era %v, server %+v, instructions %q", cl.Era(), cl.Server(), cl.Instructions())
	}
	list, err := cl.ListTools(ctx)
	if err != nil || len(list.Tools) != 1 {
		t.Fatalf("tools/list = %+v, %v", list, err)
	}
	_ = cl.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run after EOF: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not end when stdin closed")
	}
	_ = outR.Close()
}

// nopAsker stands in for the Gemini client.
type nopAsker struct{}

func (nopAsker) Ask(context.Context, string) (string, error) { return "", nil }
