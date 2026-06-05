package vertexai

import (
	"context"
	"errors"
	"testing"

	"github.com/nlink-jp/ask-gemini-mcp/internal/toolerr"
)

func TestIsRetryable_KnownTransients(t *testing.T) {
	for _, m := range []string{
		"got 429 too many requests",
		"503 service unavailable",
		"connection reset by peer",
		"i/o deadline exceeded talking to upstream",
		"unexpected EOF",
	} {
		if !isRetryable(errors.New(m)) {
			t.Errorf("isRetryable(%q) = false, want true", m)
		}
	}
}

func TestIsRetryable_NonTransients(t *testing.T) {
	for _, m := range []string{
		"401 unauthorized",
		"invalid argument: prompt too short",
		"permission denied: missing iam role",
	} {
		if isRetryable(errors.New(m)) {
			t.Errorf("isRetryable(%q) = true, want false", m)
		}
	}
}

func TestIsRetryable_RejectsContextErrors(t *testing.T) {
	// Context deadline / cancel are not transient — there is nothing
	// for a backoff to fix. They must surface as upstream_timeout.
	if isRetryable(context.DeadlineExceeded) {
		t.Errorf("isRetryable(DeadlineExceeded) = true")
	}
	if isRetryable(context.Canceled) {
		t.Errorf("isRetryable(Canceled) = true")
	}
}

func TestClassifyError_TimeoutAndCancelMapToUpstreamTimeout(t *testing.T) {
	for _, in := range []error{context.DeadlineExceeded, context.Canceled} {
		got := classifyError(in)
		if !errors.Is(got, toolerr.New(toolerr.CodeUpstreamTimeout, "")) {
			t.Errorf("classifyError(%v) = %v, want CodeUpstreamTimeout", in, got)
		}
	}
}

func TestClassifyError_OthersMapToUpstreamError(t *testing.T) {
	got := classifyError(errors.New("schema mismatch"))
	if !errors.Is(got, toolerr.New(toolerr.CodeUpstreamError, "")) {
		t.Errorf("classifyError = %v, want CodeUpstreamError", got)
	}
}

func TestClassifyError_NilPassthrough(t *testing.T) {
	if got := classifyError(nil); got != nil {
		t.Errorf("classifyError(nil) = %v, want nil", got)
	}
}
