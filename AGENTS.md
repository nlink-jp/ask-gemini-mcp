# AGENTS.md — ask-gemini-mcp

## Project summary

MCP stdio server exposing `ask_gemini(prompt: string)`, forwarding to
Vertex AI Gemini. Single-tool, stateless. Go, single binary.

## Build

```sh
make build        # → dist/ask-gemini-mcp
make build-all    # 5 platforms
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
│   ├── jsonrpc/             Request / Response / Error
│   ├── transport/           stdio line-delimited JSON
│   ├── mcpserver/           initialize / tools/list / tools/call
│   ├── toolerr/             {Code, Message, Details}
│   ├── vertexai/            google.golang.org/genai wrapper + backoff
│   └── tools/               ask_gemini.go (MCP tool handler)
├── e2e/                     //go:build e2e: harness + lifecycle / error tests
├── docs/
│   ├── en/                  README, RFP (no language suffix)
│   └── ja/                  README, RFP (.ja.md suffix)
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
- **MCP has no protocol-level cancel**: closing stdin is the only way to
  signal cancellation. The server detects this via EOF and cancels the
  parent context, aborting in-flight Gemini calls.
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
  file was found in `internal/transport` on 2026-09-21 and left alone as
  unrelated). Run `go vet ./...` and `gofmt -l .` by hand before a release.
