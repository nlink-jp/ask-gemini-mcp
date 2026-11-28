# AGENTS.md — ask-gemini-mcp

## Project summary

MCP stdio server exposing `ask_gemini(prompt: string)`, forwarding to
Vertex AI Gemini. Single-tool, stateless. Go, single binary.

## Build

```sh
make build        # → dist/ask-gemini-mcp
make build-all    # 5 platforms
make vuln            # govulncheck over the build; `make package` runs it first and stops on a finding
make verify-release  # gate: notarized, fresh, runs at this version, clean linux archives (run before upload)
```

`make build` MUST be used — never `go build` (drops binary in project root).

## Test

```sh
make test         # unit tests
make test-e2e     # builds + spawns binary + drives over stdio (real Vertex AI)
```

E2E requires `ASK_GEMINI_PROJECT` set and `gcloud auth application-default login`.

## Repo structure

```
ask-gemini-mcp/
├── main.go
├── cmd/root.go              Cobra root, --config / --version
├── internal/
│   ├── config/              TOML + env loader
│   ├── transport/           holds stdin/stdout (framing is nlink-jp/mcp's)
│   ├── mcpserver/           thin layer over nlink-jp/mcp's server.ToolServer (ADR-0001)
│   ├── toolerr/             {Code, Message, Details}
│   ├── vertexai/            google.golang.org/genai wrapper + backoff
│   └── tools/               ask_gemini.go (MCP tool handler)
├── e2e/                     //go:build e2e: harness + lifecycle / error tests
├── docs/
│   ├── en/                  RFP, adr/ (no language suffix)
│   └── ja/                  RFP, adr/ (.ja.md suffix)
├── config.example.toml
├── Makefile / .gitignore / go.mod / go.sum
├── README.md / README.ja.md
├── CHANGELOG.md / LICENSE
└── CLAUDE.md / AGENTS.md
```

## Gotchas

- **stdout is sacred**: the MCP protocol speaks JSON-RPC over stdout. All
  logging goes to stderr via `log/slog`. `fmt.Println` is banned.
- **Stateless**: each `ask_gemini` call is independent. No session state
  is kept server-side; the MCP client maintains conversation history.
- **Cancellation is `notifications/cancelled`** (ADR-0001): the module
  cancels the call's ctx, the Gemini request in flight stops, no retry
  follows, and the call gets no answer. Closing stdin is not a
  cancellation: at EOF every call that arrived is answered first.
- **The MCP protocol is nlink-jp/mcp's** (ADR-0001, organization ADR-030
  §6). Nothing outside the module handles `initialize` or
  `server/discover`; the e2e harness opens the binary with the module's
  `client`. An unknown tool is -32602 — the module's rule.
- **Each call is logged** (`tool call begin` / `ok` / `failed` with
  latency) by `internal/mcpserver`'s `call`; keep it there when the layer
  changes.
- **Retryable errors only**: `isRetryable()` matches well-known transient
  failure substrings (429 / 5xx / connection / timeout). Auth and schema
  errors return immediately so the user sees the real cause.
- **Every tool schema is closed** (`additionalProperties: false`, organization
  ADR-021 §10), and both halves of the contract are real: the schema stops a
  mistyped argument at a validating client, the handler's
  `DisallowUnknownFields` stops it at the server. `ask_gemini` already set the
  key; `TestEveryToolSchemaIsClosed` is what keeps it set and what covers the
  next tool.
- **`tools.Registry` is the single registration list.** `cmd` registers from
  it and the arch test walks it, so a tool added there is registered and
  asserted over without touching either caller. Do not go back to naming a
  tool constructor directly in `cmd` — a second list is a list that drifts,
  and the arch test would then be asserting over a set the server does not
  serve.
- **No `make check` / `make lint` target** — only `make test`. Nothing in this
  repo gates formatting, so `gofmt` drift accumulates unnoticed (one such
  file was found in `internal/transport` on 2026-09-21; the migration to
  nlink-jp/mcp rewrote it). Run `go vet ./...` and `gofmt -l .` by hand before a release.
