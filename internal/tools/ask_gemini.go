// Package tools registers the MCP tools exposed by ask-gemini-mcp.
//
// The server today exposes exactly one tool, ask_gemini, that
// forwards a single prompt to Vertex AI Gemini and returns the
// response. Use-case-specific tools (review / discuss / factcheck …)
// were deliberately not added — keeping the tool surface minimal
// matches the project's stateless transparent-pipe model.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/nlink-jp/ask-gemini-mcp/internal/mcpserver"
	"github.com/nlink-jp/ask-gemini-mcp/internal/toolerr"
)

// Asker is the abstraction the handler depends on. Production wires
// it to *vertexai.Client; tests substitute a fake.
type Asker interface {
	Ask(ctx context.Context, prompt string) (string, error)
}

// askGeminiInputSchema is the MCP inputSchema advertised in
// tools/list. Clients (Claude Desktop / Code) validate arguments
// against this before calling.
var askGeminiInputSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "prompt": {
      "type": "string",
      "description": "The question or consultation to send to Gemini. Free-form text. Include relevant context, background, and your current thinking — Gemini does not see the surrounding conversation."
    }
  },
  "required": ["prompt"],
  "additionalProperties": false
}`)

const askGeminiDescription = `Ask Gemini for a second opinion. ` +
	`Forwards the prompt to Vertex AI Gemini and returns its response. ` +
	`Stateless: each call is independent. Include any context needed in the prompt itself.`

// AskGeminiTool returns the MCP tool descriptor.
func AskGeminiTool() mcpserver.Tool {
	return mcpserver.Tool{
		Name:        "ask_gemini",
		Description: askGeminiDescription,
		InputSchema: askGeminiInputSchema,
	}
}

// askGeminiArgs is the JSON shape of the tool arguments. Tagged as
// strict-decode (unknown fields rejected) so a misspelled key surfaces
// as an invalid_arguments error rather than silently being ignored
// (feedback_strict_json_decode.md).
type askGeminiArgs struct {
	Prompt string `json:"prompt"`
}

// AskGeminiHandler returns an MCP tool handler bound to asker. Wire
// the returned function into mcpserver via RegisterTool.
func AskGeminiHandler(asker Asker) mcpserver.ToolHandler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args askGeminiArgs
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&args); err != nil {
			return "", toolerr.New(toolerr.CodeInvalidArguments, "invalid arguments: "+err.Error())
		}
		if strings.TrimSpace(args.Prompt) == "" {
			return "", toolerr.New(toolerr.CodeInvalidArguments, "prompt must not be empty")
		}
		return asker.Ask(ctx, args.Prompt)
	}
}
