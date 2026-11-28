# ADR-0001: The MCP protocol layer becomes nlink-jp/mcp — both eras

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-28) |
| Date | 2026-11-28 |
| Binds | ask-gemini-mcp |
| Decision makers | nlink-jp maintainers |
| Triggered by | Organization ADR-030 (every MCP implementation speaks both eras and takes its protocol layer only from nlink-jp/mcp); issue nlink-jp/ask-gemini-mcp#1 |

This is the repository's first ADR; earlier decisions live in the RFP (`docs/ja/ask-gemini-mcp-rfp.ja.md`). Its twin ask-llm-mcp
takes the same decision in its own ADR-0001.

## Context

`internal/mcpserver`, `internal/transport` and `internal/jsonrpc` were a hand-written stdio MCP server, fixed at
2024-11-05 and speaking the legacy era only (the `initialize` handshake), so a modern-only client of the 2026-07-28
revision could not reach it. ask-gemini-mcp belongs to family C (mcpserver; nlink-jp/mcp's server-migration §1), whose reference
implementation is data-toolbox-mcp (its ADR-0013). The re-measurement of the real peers (2026-11-27, ADR-030 §7) is
recorded in asn-lookup's ADR-0001: Claude Code 2.1.311, gem-agent and lagent are all dual-era.

## Decision

Follow the common policy (nlink-jp/mcp's `docs/en/server-migration.md`, v0.3.0), with these items for this
repository:

1. **The shape seen from outside stays (policy §2).** `mcpserver.New(name, version, transport, logger)`,
   `RegisterTool`, `Tool`, `ToolHandler` (which returns `(string, error)`) and `ContentBlock` stay; nothing is added.
   `Serve(ctx)` calls `AddTool` in registration order, then `ToolServer.Serve`. A handler's text becomes one text
   block, a `*toolerr.Error` its `{code, message, details}` JSON as an isError, any other error its text as an
   isError. Each call is still logged (`tool call begin`, then `tool call ok` with latency and bytes, or `tool call
   failed` with latency and the error). The server sets no instructions, as before.
2. **Transport (policy §2)**: `transport.NewStdioTransport(in, out)` stays as the holder of the two streams (`In`,
   `Out`); its 1 MiB `bufio.Scanner`, read/write code and its tests go. `internal/jsonrpc`, used only by the protocol
   layer, goes.
3. **Options (policy §2)**: `Workers` 1 (calls were served one at a time), `QueueDepth` 256; the logger is the one
   `cmd` builds on stderr (without one, `New` logs Warn and above to stderr).
4. **Behaviour that changes (policy §4)**:
   - An unknown tool is -32602 instead of -32601; `ping` gets `{}` instead of -32601.
   - `tools/*` before initialize is -32602, and so are `arguments` that is not an object.
   - A request with `"id":null` gets -32600 (it was silent, as a notification).
   - The line cap goes from 1 MiB to 16 MiB; a line over it gets -32600 and reading goes on (it stopped). A blank line
     is no longer answered (it got -32700).
   - A failed write to stdout stops the server (it was logged and reading went on).
   - A panic in a tool is answered -32603 and the server goes on (it ended the process).
   - The version moves from a fixed 2024-11-05 to legacy negotiation and the modern era.
   - SIGINT and SIGTERM (`signal.NotifyContext`) cancel the server's ctx, even mid-read: the running call is
     cancelled and `Serve` returns within the module's shutdown grace (5 s); it returns nil, so the caller's
     `context.Canceled` check goes.
5. **Cancellation (policy §5)**: `notifications/cancelled` cancels the call's ctx. The request to Vertex AI Gemini in
   flight stops, no retry follows (a cancellation was already not retryable), and the module sends no answer for the
   cancelled call. Whatever the upstream had already received is spent.
6. **Dependency**: `github.com/nlink-jp/mcp` is added.
7. **Version**: minor (v0.3.0).

## Defects found (fixed in their own fix commit)

- `ask_gemini` refused an undeclared argument but took a declared one spelt in another case (`Prompt` for `prompt`) and
  asked the model with it, since encoding/json matches names case-insensitively (organization ADR-021 §4). It now
  refuses that by name, with a test that fails without the fix.
- AGENTS.md and CLAUDE.md said closing stdin cancels the in-flight call. It never did: the server read its next line
  only after the running call returned, so EOF was noticed after it. They now say what happens: closing stdin answers
  every call that arrived first; `notifications/cancelled` is the cancellation.

## Consequences

- Modern-only clients reach it too; legacy clients are answered as before.
- Tests run through the module's `client` (`internal/mcpserver`, `cmd`); a test against the three probe clients was
  added. The Go e2e (`e2e/`, `make test-e2e`) opens the binary with the module's client instead of writing the
  handshake itself, so it no longer needs the ADR-030 §6 exemption it was listed for (to be dropped from the
  request, issue nlink-jp/.github#2).

## Alternatives considered

| Alternative | Why not |
|---|---|
| Drop `mcpserver` and register the tool on the module directly | The tool's and tests' registration would change; the thin layer leaves the tool unchanged |
| Give ask-llm-mcp the same `newAsker` seam ask-gemini-mcp's `cmd` gained | ask-gemini needs it because the Gemini SDK resolves credentials when the client is made, and a wiring test must need none; ask-llm's `openai.New` makes no call, so the twins differ here on purpose |
| Let `ToolHandler` return `any`, as data-toolbox-mcp's does | A change to the tool for no gain; the layer turns the text into a block itself |

## References

- Organization ADR-030, ADR-025, ADR-021 §4
- nlink-jp/mcp v0.3.0: `docs/en/server-migration.md`
- data-toolbox-mcp ADR-0013 (family C's reference implementation)
- asn-lookup ADR-0001 (the re-measured peers)
- Issue nlink-jp/ask-gemini-mcp#1
