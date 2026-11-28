package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	mcpsrv "github.com/nlink-jp/mcp/server"

	"github.com/nlink-jp/ask-gemini-mcp/internal/toolerr"
)

// ContentBlock is one block in the tools/call result.content array.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type toolsCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// call runs one tool handler and turns what it returned into the protocol's
// result: the text as one text block, and an error into an isError result
// (toolError). Each call is logged with its latency.
func call(ctx context.Context, h ToolHandler, name string, args json.RawMessage, logger *slog.Logger) (*mcpsrv.Result, error) {
	start := time.Now()
	logger.Debug("tool call begin", "tool", name)
	out, err := h(ctx, args)
	latency := time.Since(start).Round(time.Millisecond)
	if err != nil {
		logger.Info("tool call failed", "tool", name, "latency", latency, "err", err)
		return result(toolError(err))
	}
	logger.Info("tool call ok", "tool", name, "latency", latency, "bytes", len(out))
	return result(toolsCallResult{Content: []ContentBlock{{Type: "text", Text: out}}})
}

// result copies a tools/call payload into the protocol's shape.
func result(r toolsCallResult) (*mcpsrv.Result, error) {
	out := &mcpsrv.Result{IsError: r.IsError}
	for _, b := range r.Content {
		raw, err := json.Marshal(b)
		if err != nil {
			return nil, err
		}
		out.Content = append(out.Content, raw)
	}
	return out, nil
}

// toolError is a tool error per MCP convention: a result with
// isError=true and a single text content block. If err is (or wraps) a
// *toolerr.Error, the content carries the structured {code, message,
// details} JSON so LLM clients can branch on the code. Otherwise the
// plain Error() string is used.
func toolError(err error) toolsCallResult {
	var te *toolerr.Error
	if errors.As(err, &te) {
		if body, marshalErr := json.Marshal(te); marshalErr == nil {
			return toolsCallResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: string(body)}}}
		}
		// Fall through to plain text on marshal failure.
	}
	return toolsCallResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}
}
