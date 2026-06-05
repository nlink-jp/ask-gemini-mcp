// Package vertexai is a thin Vertex AI Gemini client used by
// ask-gemini-mcp's MCP tool. It wraps google.golang.org/genai with a
// minimal surface — Ask(prompt) returns text — and isolates the SDK
// dependency so the tool layer can take an Asker interface instead of
// a concrete client.
//
// Phase 1 has no retry / backoff; transient failures surface
// immediately. Phase 2 layers nlk/backoff on top of the same Ask().
package vertexai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nlink-jp/nlk/backoff"
	"google.golang.org/genai"

	"github.com/nlink-jp/ask-gemini-mcp/internal/toolerr"
)

// maxRetries caps the number of retry attempts on retryable failures
// (429 / 5xx / transport). Beyond this we surface the last error to
// the caller so the MCP client sees the real cause rather than a
// generic timeout from the backoff loop.
const maxRetries = 5

// Client is the per-process Gemini handle for ask-gemini-mcp. One
// instance is shared across every ask_gemini tool call.
type Client struct {
	inner   *genai.Client
	model   string
	timeout time.Duration
	logger  *slog.Logger
}

// SetLogger attaches a logger used for retry diagnostics. Optional:
// the zero-value Client logs to a discard sink.
func (c *Client) SetLogger(l *slog.Logger) {
	if l != nil {
		c.logger = l
	}
}

// New creates a Vertex AI Gemini client. timeoutSec caps each
// underlying GenerateContent call; pass 0 to use the SDK default.
func New(ctx context.Context, project, location, model string, timeoutSec int) (*Client, error) {
	if project == "" {
		return nil, fmt.Errorf("vertex AI client: project is required")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  project,
		Location: location,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex AI client: %w", err)
	}
	c := &Client{
		inner: client,
		model: model,
	}
	if timeoutSec > 0 {
		c.timeout = time.Duration(timeoutSec) * time.Second
	}
	return c, nil
}

// Model returns the configured model name.
func (c *Client) Model() string {
	return c.model
}

// Ask sends prompt to Gemini and returns the response text.
//
// Cancellation: the inbound ctx is the Cobra signal context, so
// SIGINT/SIGTERM aborts any in-flight call. On top of that, the
// per-call timeout from config (Model.RequestTimeout, default 180s)
// is layered via context.WithTimeout so a single hung upstream call
// can never block the MCP client forever.
//
// Retry: transient failures (429 / 5xx / transport) are retried with
// exponential backoff up to maxRetries. Non-retryable failures
// (auth, schema) surface immediately so the MCP client sees the
// real cause.
//
// An empty response is mapped to CodeUpstreamError; an exceeded
// per-call timeout (or upstream cancellation) is mapped to
// CodeUpstreamTimeout so the MCP client can branch on either.
func (c *Client) Ask(ctx context.Context, prompt string) (string, error) {
	callCtx := ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	contents := []*genai.Content{
		genai.NewContentFromText(prompt, "user"),
	}
	cfg := &genai.GenerateContentConfig{
		// Thoughts are an internal Gemini reasoning artifact we do
		// not surface to the MCP client. Defence-in-depth: also
		// filter Thought parts in extractText.
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: false,
		},
	}

	bo := backoff.New(
		backoff.WithBase(2*time.Second),
		backoff.WithMax(30*time.Second),
	)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.inner.Models.GenerateContent(callCtx, c.model, contents, cfg)
		if err == nil {
			if filtErr := inspectFinishReason(resp); filtErr != nil {
				// A content-filter block is not a transient
				// failure; retrying produces the same answer.
				return "", filtErr
			}
			text := extractText(resp)
			if text == "" {
				return "", toolerr.New(toolerr.CodeUpstreamError, "empty response from Gemini")
			}
			return text, nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == maxRetries {
			return "", classifyError(err)
		}
		wait := bo.Duration(attempt)
		c.log().Warn("vertex AI call failed, retrying",
			"attempt", attempt+1, "max", maxRetries+1,
			"wait", wait.Round(time.Second), "err", err)
		select {
		case <-time.After(wait):
		case <-callCtx.Done():
			return "", classifyError(callCtx.Err())
		}
	}
	return "", classifyError(lastErr)
}

func (c *Client) log() *slog.Logger {
	if c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

// classifyError maps low-level errors into structured tool errors.
// Anything that looks like a deadline or cancellation becomes
// CodeUpstreamTimeout; everything else becomes CodeUpstreamError with
// the inner message attached.
func classifyError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return toolerr.New(toolerr.CodeUpstreamTimeout, "gemini call exceeded per-request timeout").
			WithDetails(map[string]any{"cause": err.Error()})
	}
	if errors.Is(err, context.Canceled) {
		return toolerr.New(toolerr.CodeUpstreamTimeout, "gemini call cancelled").
			WithDetails(map[string]any{"cause": err.Error()})
	}
	return toolerr.New(toolerr.CodeUpstreamError, fmt.Sprintf("gemini generate: %v", err))
}

// isRetryable classifies whether the error is worth retrying.
// Conservative: only well-known transient failure substrings trigger a
// retry; anything else (auth failures, bad-request schema errors)
// returns immediately so the user sees the real cause rather than
// spinning the back-off.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"429", "503", "500", "502",
		"unavailable", "deadline", "timeout",
		"connection refused", "eof", "reset by peer",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// inspectFinishReason returns a non-nil structured error if the
// candidate was terminated by a content filter or other non-recoverable
// upstream cause. STOP / MAX_TOKENS / empty (no candidates) are all
// treated as normal — empty-text handling lives in the caller.
//
// The MCP client can branch on details["category"]: "content_filter",
// "recitation", or "other".
func inspectFinishReason(resp *genai.GenerateContentResponse) error {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil
	}
	fr := resp.Candidates[0].FinishReason
	switch fr {
	case "", genai.FinishReasonStop, genai.FinishReasonMaxTokens, genai.FinishReasonUnspecified:
		return nil
	case genai.FinishReasonSafety,
		genai.FinishReasonProhibitedContent,
		genai.FinishReasonBlocklist,
		genai.FinishReasonSPII,
		genai.FinishReasonImageSafety,
		genai.FinishReasonImageProhibitedContent:
		return toolerr.New(toolerr.CodeUpstreamError, "gemini blocked the prompt or response").
			WithDetails(map[string]any{
				"finish_reason": string(fr),
				"category":      "content_filter",
			})
	case genai.FinishReasonRecitation, genai.FinishReasonImageRecitation:
		return toolerr.New(toolerr.CodeUpstreamError, "gemini blocked output for recitation").
			WithDetails(map[string]any{
				"finish_reason": string(fr),
				"category":      "recitation",
			})
	default:
		return toolerr.New(toolerr.CodeUpstreamError, "gemini terminated with non-normal reason").
			WithDetails(map[string]any{
				"finish_reason": string(fr),
				"category":      "other",
			})
	}
}

// extractText pulls the text out of a Gemini response, filtering
// Thought parts at the structural level.
func extractText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return ""
	}
	var parts []string
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.Thought {
			continue
		}
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "")
}
