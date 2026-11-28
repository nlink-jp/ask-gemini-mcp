# ask-gemini-mcp

[日本語版](README.ja.md)

A Model Context Protocol (MCP) server that exposes a single tool,
`ask_gemini(prompt)`, which forwards the prompt to Vertex AI Gemini
and returns the response.

Intended as a second-opinion channel for AI coding agents
(Claude Code, Claude Desktop, …) — especially useful in MCP clients
without shell access, where the existing `gem-*` CLI tools cannot be
invoked directly.

## Status

Released. The MCP server is live and exposes a single tool, `ask_gemini`.
See [CHANGELOG.md](CHANGELOG.md) for the current version and
[`docs/en/ask-gemini-mcp-rfp.md`](docs/en/ask-gemini-mcp-rfp.md) for the
design RFP.

The server speaks both eras of the MCP protocol: the 2026-07-28 revision
(clients that probe with `server/discover`) and the earlier `initialize`
handshake (2025-11-25, 2025-06-18, 2024-11-05), through the organization's
module [nlink-jp/mcp](https://github.com/nlink-jp/mcp). A client that cancels a
call stops the Gemini request in flight. Requests piped in are all answered
before the server exits at end of input.
([ADR-0001](docs/en/adr/0001-mcp-through-nlink-jp-mcp.md))

## Quick start

```sh
# Install
go install github.com/nlink-jp/ask-gemini-mcp@latest

# Authenticate Vertex AI via Application Default Credentials.
gcloud auth application-default login

# Configure
mkdir -p ~/.config/ask-gemini-mcp
cp config.example.toml ~/.config/ask-gemini-mcp/config.toml
$EDITOR ~/.config/ask-gemini-mcp/config.toml  # set [gcp].project
```

Then register the server with your MCP client. For Claude Desktop, add
to `~/.config/claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ask-gemini": {
      "command": "/path/to/ask-gemini-mcp"
    }
  }
}
```

For Claude Code, run:

```sh
claude mcp add ask-gemini /path/to/ask-gemini-mcp
```

## Configuration

Settings load order: built-in defaults → TOML file → env vars.

- **Config file**: `~/.config/ask-gemini-mcp/config.toml` (or `-c` flag)
- **Env vars**: `ASK_GEMINI_*` (tool-specific) > `GOOGLE_CLOUD_*` (generic fallback)

| Variable                  | Required | Default              |
|---------------------------|----------|----------------------|
| `ASK_GEMINI_PROJECT`      | Yes      | —                    |
| `ASK_GEMINI_LOCATION`     | No       | `us-central1`        |
| `ASK_GEMINI_MODEL`        | No       | `gemini-2.5-flash`   |

## Build

```sh
make build      # → dist/ask-gemini-mcp
make test       # go test ./...
make build-all  # cross-compile 4 platforms (darwin arm64 only; no Intel)
```

## License

[MIT](LICENSE)
