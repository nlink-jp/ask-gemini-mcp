// Package cmd defines the ask-gemini-mcp CLI surface via cobra.
//
// ask-gemini-mcp is a Vertex AI Gemini consultation MCP server. It
// speaks the MCP protocol over stdio and exposes a single tool,
// ask_gemini, that MCP clients (Claude Code / Desktop etc.) can use
// to ask Gemini for a second opinion on design questions, code, etc.
package cmd

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/nlink-jp/ask-gemini-mcp/internal/config"
	"github.com/nlink-jp/ask-gemini-mcp/internal/mcpserver"
	"github.com/nlink-jp/ask-gemini-mcp/internal/tools"
	"github.com/nlink-jp/ask-gemini-mcp/internal/transport"
	"github.com/nlink-jp/ask-gemini-mcp/internal/vertexai"
)

// logLevel parses ASK_GEMINI_LOG_LEVEL; an unknown or empty value
// falls back to slog.LevelInfo so a misspelled env var does not
// silently lose all visibility.
func logLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

var (
	flagConfig    string
	serverVersion = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "ask-gemini-mcp",
	Short: "MCP server that forwards prompts to Vertex AI Gemini",
	Long: `ask-gemini-mcp is an MCP stdio server that exposes a single tool,
ask_gemini(prompt), which forwards the prompt to Vertex AI Gemini and
returns the response. Intended as a second-opinion channel for AI
coding agents (Claude Code, Claude Desktop, etc.) — especially useful
in MCP clients that have no shell access and therefore cannot use the
gem-* CLI tools directly.

Configuration lives at ~/.config/ask-gemini-mcp/config.toml; see
config.example.toml for the schema. Override the path with --config.`,
	RunE: run,
}

// Execute runs the root command. Called from main.go with the
// build-time version string injected via -ldflags.
func Execute(version string) {
	serverVersion = version
	rootCmd.Version = version
	rootCmd.Flags().StringVarP(&flagConfig, "config", "c", "",
		"Config file path (default: ~/.config/ask-gemini-mcp/config.toml)")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// run wires the MCP server: load config, create the Vertex AI client,
// register the ask_gemini tool, and serve over stdio until SIGINT /
// SIGTERM or stdin EOF.
//
// Logging goes to stderr only. stdout is owned by the MCP transport;
// anything else written there would break the JSON-RPC framing.
func run(cmd *cobra.Command, args []string) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel(os.Getenv("ASK_GEMINI_LOG_LEVEL")),
	}))

	cfg, err := config.Load(flagConfig)
	if err != nil {
		return err
	}

	ctx, cancel := notifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	asker, err := newAsker(ctx, cfg, logger)
	if err != nil {
		return err
	}

	logger.Info("ask-gemini-mcp ready",
		"project", cfg.GCP.Project,
		"location", cfg.GCP.Location,
		"model", cfg.Model.Name,
		"request_timeout_s", cfg.Model.RequestTimeout)

	srv := newServer(transport.NewStdioTransport(serveStdin, serveStdout), logger, asker)
	// Serve returns nil for EOF and for a signal's cancellation.
	return srv.Serve(ctx)
}

// notifyContext is signal.NotifyContext, serveStdin and serveStdout the
// process's streams, and newAsker the Gemini client's constructor; tests
// stand in for them, so a test of the wiring needs no Google credentials.
var (
	notifyContext           = signal.NotifyContext
	serveStdin    io.Reader = os.Stdin
	serveStdout   io.Writer = os.Stdout
	newAsker                = func(ctx context.Context, cfg *config.Config, logger *slog.Logger) (tools.Asker, error) {
		client, err := vertexai.New(ctx, cfg.GCP.Project, cfg.GCP.Location, cfg.Model.Name, cfg.Model.RequestTimeout)
		if err != nil {
			return nil, err
		}
		client.SetLogger(logger)
		return client, nil
	}
)

// newServer builds the MCP server that run serves. It is the only place in
// this package that constructs one, so a test that drives it sees what the
// binary answers.
func newServer(tr *transport.StdioTransport, logger *slog.Logger, asker tools.Asker) *mcpserver.Server {
	srv := mcpserver.New("ask-gemini-mcp", serverVersion, tr, logger)
	for _, r := range tools.Registry(asker) {
		srv.RegisterTool(r.Tool, r.Handler)
	}
	return srv
}
