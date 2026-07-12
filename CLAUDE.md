# CLAUDE.md — ask-gemini-mcp

**Organization rules (mandatory): https://github.com/nlink-jp/.github/blob/main/CONVENTIONS.md**

## Project overview

MCP stdio server that exposes a single tool `ask_gemini(prompt: string)`
forwarding prompts to Vertex AI Gemini and returning the response. Use
case: AI coding agents (Claude Code, Claude Desktop, …) consulting a
different model for a second opinion — especially in MCP clients without
shell access, where `gem-*` CLI tools cannot be used directly.

## Non-negotiable rules

- **Tests are mandatory** — write them with the implementation
- **Never `go build` directly** — always `make build` (outputs to `dist/`)
- **Docs in sync** — update `README.md` and `README.ja.md` together
- **Small, typed commits** — `feat:`, `fix:`, `test:`, `chore:`, `docs:`, `refactor:`, `security:`
- **stdout is sacred** — the MCP transport runs over stdout; logs MUST go to stderr
- **Stateless** — no conversation history; every `ask_gemini` call is independent

## Build & test

```sh
make build      # → dist/ask-gemini-mcp (auto-codesigns on darwin if a
                # Developer ID Application identity is in the keychain)
make test       # go test ./...
make build-all  # cross-compile 4 platforms (darwin arm64 only); darwin gets codesigned
make test-e2e   # build + spawn binary + drive over stdio (needs Vertex AI auth)
make package    # build-all + zip with version suffix + notarize darwin zips
                # via NOTARY_PROFILE (default: nlink-jp-notary)
```

Both signing and notarization degrade gracefully: missing keychain
identity / notary profile produce un-signed / un-notarized binaries
with a one-line warning. Contributors without an Apple Developer
Program account can still build.

## Configuration

Settings load order: built-in defaults → TOML file → env vars.

- **Config file**: `~/.config/ask-gemini-mcp/config.toml` (or `-c` flag)
- **Env vars**: `ASK_GEMINI_*` (tool-specific) > `GOOGLE_CLOUD_*` (generic fallback)

| Variable                       | Required | Default              |
|--------------------------------|----------|----------------------|
| `ASK_GEMINI_PROJECT`           | Yes      | —                    |
| `ASK_GEMINI_LOCATION`          | No       | `us-central1`        |
| `ASK_GEMINI_MODEL`             | No       | `gemini-2.5-flash`   |
| `ASK_GEMINI_REQUEST_TIMEOUT`   | No       | `180` (seconds)      |
| `ASK_GEMINI_LOG_LEVEL`         | No       | `info`               |

## Key dependencies

- `google.golang.org/genai` — Vertex AI Gemini SDK
- `github.com/nlink-jp/nlk/backoff` — Vertex AI retry backoff
- `github.com/spf13/cobra` — CLI framework
- `github.com/BurntSushi/toml` — config file parsing

## Architecture

- `cmd/` — Cobra root command (flat, no subcommand), wires stdio server
- `internal/config/` — TOML + env-var configuration
- `internal/jsonrpc/` — JSON-RPC 2.0 message types
- `internal/transport/` — line-delimited JSON stdio transport
- `internal/mcpserver/` — MCP protocol: initialize / tools/list / tools/call
- `internal/toolerr/` — structured `{code, message, details}` errors
- `internal/vertexai/` — thin Gemini client (genai SDK + retry)
- `internal/tools/` — MCP tool handlers (`ask_gemini.go`)
- `e2e/` — `//go:build e2e` harness driving the binary over stdio

## Gotchas

- **stdout collisions**: anything written to stdout that is not a JSON-RPC
  message will break the MCP client. Use `log/slog` to stderr only;
  `fmt.Println` is banned.
- **No retry for non-transient errors**: `isRetryable()` matches well-known
  transient failure substrings only. Auth / schema errors return immediately.
- **Context cancel**: when the client closes stdin, the parent context is
  cancelled and in-flight Gemini calls abort (MCP 2024-11-05 has no
  protocol-level cancel notification — closing stdin is the only signal).

## Design references

- [`docs/en/ask-gemini-mcp-rfp.md`](docs/en/ask-gemini-mcp-rfp.md) /
  [`docs/ja/ask-gemini-mcp-rfp.ja.md`](docs/ja/ask-gemini-mcp-rfp.ja.md)
  — approved design RFP; canonical source for scope decisions.
