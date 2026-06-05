# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-06-06

### Added

- Scaffolded project skeleton: Cobra root command, Makefile,
  README (EN/JA), MIT LICENSE, config.example.toml.
- Approved RFP at `docs/{en,ja}/ask-gemini-mcp-rfp*.md`.
- `internal/config`: TOML loader with strict decode and env overrides
  (`ASK_GEMINI_*` / `GOOGLE_CLOUD_*` fallback).
- `internal/jsonrpc` + `internal/transport`: JSON-RPC 2.0 types and
  line-delimited stdio transport (1MB buffer, mutex-serialised write).
- `internal/mcpserver`: MCP protocol (2024-11-05) — initialize,
  tools/list, tools/call, structured error envelope.
- `internal/toolerr`: `{code, message, details}` with `errors.Is`
  by code; sentinel codes `invalid_arguments`, `upstream_error`,
  `upstream_timeout`, `internal_error`.
- `internal/vertexai`: Vertex AI Gemini client via
  `google.golang.org/genai` with `nlk/backoff` retry (Base=2s, Max=30s,
  max 5 retries) on known transient failures.
- `internal/tools`: single MCP tool `ask_gemini(prompt: string)` with
  strict argument decoding and empty-prompt rejection.
- `cmd/root.go`: end-to-end wiring with `signal.NotifyContext` for
  SIGINT/SIGTERM propagation to Gemini calls.
- Per-call timeout via `[model].request_timeout` (default 180s) with
  `context.WithTimeout`; deadline/cancel mapped to `upstream_timeout`.
- Content-filter and recitation block detection from Gemini's
  `FinishReason`, surfaced as `upstream_error` with
  `details.category` = `content_filter` / `recitation` / `other`.
- `log/slog` structured logging to **stderr only**; level via
  `ASK_GEMINI_LOG_LEVEL` (debug / info / warn / error).
- E2E test harness (`e2e/`, `//go:build e2e`) driving the built binary
  over JSON-RPC stdio: lifecycle, empty-prompt error, unknown tool,
  per-request timeout.

### Internal notes

- RFP `Configuration` section corrected: config path is per-tool
  `~/.config/ask-gemini-mcp/config.toml`, matching the existing gem-*
  convention (schema-level unification only, not path).
- Release pipeline: `scripts/codesign-darwin.sh` (Developer ID +
  Hardened Runtime + Apple timestamp) and `scripts/notarize-darwin.sh`
  (xcrun notarytool via NOTARY_PROFILE) wired into `make build` /
  `make build-all` / `make package`. Both degrade gracefully when
  the local keychain lacks the identity / profile.
