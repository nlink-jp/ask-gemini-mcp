package vertexai

import (
	"errors"
	"testing"

	"google.golang.org/genai"

	"github.com/nlink-jp/ask-gemini-mcp/internal/toolerr"
)

func candWithFinish(fr genai.FinishReason) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{FinishReason: fr}},
	}
}

func TestInspectFinishReason_NormalReasonsReturnNil(t *testing.T) {
	for _, fr := range []genai.FinishReason{
		"", genai.FinishReasonStop, genai.FinishReasonMaxTokens, genai.FinishReasonUnspecified,
	} {
		if err := inspectFinishReason(candWithFinish(fr)); err != nil {
			t.Errorf("finish %q: got error %v, want nil", fr, err)
		}
	}
}

func TestInspectFinishReason_SafetyBlocksAreClassified(t *testing.T) {
	cases := map[genai.FinishReason]string{
		genai.FinishReasonSafety:                 "content_filter",
		genai.FinishReasonProhibitedContent:      "content_filter",
		genai.FinishReasonBlocklist:              "content_filter",
		genai.FinishReasonSPII:                   "content_filter",
		genai.FinishReasonImageSafety:            "content_filter",
		genai.FinishReasonImageProhibitedContent: "content_filter",
		genai.FinishReasonRecitation:             "recitation",
		genai.FinishReasonImageRecitation:        "recitation",
		genai.FinishReasonLanguage:               "other",
		genai.FinishReasonOther:                  "other",
	}
	for fr, wantCat := range cases {
		t.Run(string(fr), func(t *testing.T) {
			err := inspectFinishReason(candWithFinish(fr))
			if err == nil {
				t.Fatalf("expected error for %q", fr)
			}
			if !errors.Is(err, toolerr.New(toolerr.CodeUpstreamError, "")) {
				t.Fatalf("want CodeUpstreamError, got %v", err)
			}
			var te *toolerr.Error
			if !errors.As(err, &te) {
				t.Fatalf("expected toolerr.Error, got %T", err)
			}
			if te.Details["category"] != wantCat {
				t.Errorf("category = %v, want %s", te.Details["category"], wantCat)
			}
			if te.Details["finish_reason"] != string(fr) {
				t.Errorf("finish_reason = %v, want %s", te.Details["finish_reason"], fr)
			}
		})
	}
}

func TestInspectFinishReason_NilAndEmpty(t *testing.T) {
	if err := inspectFinishReason(nil); err != nil {
		t.Errorf("nil response: %v", err)
	}
	if err := inspectFinishReason(&genai.GenerateContentResponse{}); err != nil {
		t.Errorf("no candidates: %v", err)
	}
}
