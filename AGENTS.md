# AGENTS.md — ask-gemini-mcp

## Project summary

MCP stdio server exposing `ask_gemini(prompt: string)`, forwarding to
Vertex AI Gemini. Single-tool, stateless. Go, single binary.

## Build

```sh
make build        # → dist/ask-gemini-mcp
make build-all    # 5 platforms
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
