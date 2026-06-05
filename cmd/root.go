// Package cmd defines the ask-gemini-mcp CLI surface via cobra.
//
// ask-gemini-mcp is a Vertex AI Gemini consultation MCP server. It
// speaks the MCP protocol over stdio and exposes a single tool,
// ask_gemini, that MCP clients (Claude Code / Desktop etc.) can use
// to ask Gemini for a second opinion on design questions, code, etc.
package cmd

import (
	"context"
	"errors"
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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	client, err := vertexai.New(ctx, cfg.GCP.Project, cfg.GCP.Location, cfg.Model.Name, cfg.Model.RequestTimeout)
	if err != nil {
		return err
	}
	client.SetLogger(logger)

	logger.Info("ask-gemini-mcp ready",
		"project", cfg.GCP.Project,
		"location", cfg.GCP.Location,
		"model", cfg.Model.Name,
		"request_timeout_s", cfg.Model.RequestTimeout)

	tr := transport.NewStdioTransport(os.Stdin, os.Stdout)
	srv := mcpserver.New("ask-gemini-mcp", serverVersion, tr, logger)
	srv.RegisterTool(tools.AskGeminiTool(), tools.AskGeminiHandler(client))

	if err := srv.Serve(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	return nil
}
