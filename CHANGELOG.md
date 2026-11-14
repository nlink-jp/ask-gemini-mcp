# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `make vuln` — and so `make package` — scans every platform in `PLATFORMS`,
  with the environment `build-all` builds each with. It scanned the host's
  build alone, so an advisory in a Linux- or Windows-only package could not
  stop a release.

## [0.2.3] - 2026-11-14

### Security

- **`golang.org/x/net` is updated from v0.38.0 to v0.62.0.** The connection
  to Vertex AI ran on x/net's own HTTP/2, not the standard library's: the
  SDK's default client configures it that way when a tool does not supply a
  transport of its own. Measured — a request through the same client
  against a local server was handled by `golang.org/x/net/http2`. Three
  advisories apply to an HTTP/2 client there (GO-2026-4918, -6610, -6611);
  each needs the server end of the connection, which is Google's endpoint
  under TLS. With v0.62.0 built by Go 1.27 the same request is handled by
  the standard library, where they are fixed from Go 1.27.2.
- `grpc` moves from v1.66.2 to v1.83.2, `x/text` to v0.43.0 and `x/crypto`
  to v0.58.0, for advisories `govulncheck` traced through code this build
  does not run (the gRPC path is taken only when S2A is switched on).
  `grpc` v1.83.2 requires `cloud.google.com/go/auth` v0.18.2, so that
  moves from v0.9.3 with it. The Vertex AI SDK is unchanged (v1.59.0).
  The new set was checked against a local server, not against the live
  endpoint.

### Changed

- **Building from source needs Go 1.27.4 or later** (`go.mod`; it was
  1.26.2), so that no build uses a standard library with the HTTP/2
  advisories above.
- **`make package` runs the vulnerability check first** (`make vuln`), so a
  release cannot be built past a finding.

## [0.2.2] - 2026-11-04

### Changed

- **The config file is read by nlink-jp/toml** (v0.1.1) instead of the
  third-party `BurntSushi/toml`, so the server carries no TOML code from
  outside the organization. The file reads as before: unknown keys are still
  refused, each named in the error.

### Fixed

- **`make verify-release` now fails closed.** Its last block chained unzip, the
  packaged binary's `--version` and `spctl` with `&&` and ended the whole chain
  in `|| true`, so a zip that did not unpack or a binary that did not run exited
  0 and the upload proceeded. Each step is now judged on its own, the packaged
  binary's `--version` must contain the tag being released, and only the
  informational `spctl` line may be ignored. Matches the org template
  (CONVENTIONS.md §Code Signing → Verifying a release).
- **The Linux archives no longer carry macOS file metadata.** macOS `tar` wrote
  each bundled file's extended attributes (`com.apple.provenance`, and a Dropbox
  attribute where the tree is synced) into the `.tar.gz` twice: as AppleDouble
  `._` members, which GNU tar extracts as stray `._<name>` files beside the real
  ones, and as `LIBARCHIVE.xattr.*` / `SCHILY.xattr.*` pax headers, which it
  reports as unknown keywords. `make package` now archives with
  `COPYFILE_DISABLE=1 tar --no-xattrs`; each setting stops one of the two.
  Archives already published still carry them; the files themselves are
  unaffected.

### Internal

- `make verify-release` also judges each Linux archive: no AppleDouble or other
  macOS metadata members — listed with `--options 'tar:!mac-ext'`, because a
  plain macOS listing folds `._` members away — no extended attributes as pax
  headers, and exactly the canonical binary, `README.md` and `LICENSE`, compared
  in the C locale.
- The Linux-archive check in `make verify-release` reads each archive's pax
  headers with Python's `tarfile` instead of grepping the decompressed stream,
  which also matched file text that names the keywords (a bundled CHANGELOG,
  for one).

## [0.2.1] - 2026-09-21

### Added

- `TestEveryToolSchemaIsClosed` — the arch test organization ADR-021 §10
  requires: every registered tool's input schema must set
  `additionalProperties: false`. `ask_gemini` already set it, so no schema
  changed; the test is what keeps it set and what covers the next tool.
- `tools.Registry` — one list pairing each tool descriptor with its handler.
  `cmd` now registers from it and the arch test walks it, so the two cannot
  disagree about which tools exist, and a tool added there needs no change in
  either caller.

## [0.2.0] - 2026-07-12

### Removed

- **darwin/amd64 (Intel) pre-built binary.** macOS releases now ship
  **arm64 only**, per the org-wide policy (darwin is Apple-Silicon only; no
  universal binaries). Intel Mac users can build from source.

### Changed

- **Linux release archives are now `.tar.gz`** (darwin/windows remain `.zip`),
  per `nlink-jp/.github` CONVENTIONS.md §Release Archive Standard.
- **`LICENSE` is now bundled** in every release archive alongside `README.md`.
- **darwin code-signature identifier** is now the canonical `ask-gemini-mcp`.

### Fixed

- Notarization surfaces the real `notarytool` error (e.g. an expired Apple
  Developer agreement / HTTP 403) instead of a misleading "profile not found".

No change to the binary's behaviour — a packaging / build-config release.

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
