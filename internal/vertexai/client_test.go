package vertexai

import (
	"testing"

	"google.golang.org/genai"
)

func TestExtractText_JoinsNonThoughtParts(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "Hello "},
						{Text: "internal reasoning", Thought: true}, // filtered
						{Text: "world"},
					},
				},
			},
		},
	}
	if got := extractText(resp); got != "Hello world" {
		t.Errorf("extractText = %q, want %q", got, "Hello world")
	}
}

func TestExtractText_EmptyResponse(t *testing.T) {
	cases := []struct {
		name string
		in   *genai.GenerateContentResponse
	}{
		{"nil", nil},
		{"no candidates", &genai.GenerateContentResponse{}},
		{"nil content", &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{Content: nil}},
		}},
		{"only thought parts", &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Parts: []*genai.Part{{Text: "thinking", Thought: true}},
				},
			}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractText(tc.in); got != "" {
				t.Errorf("extractText = %q, want empty", got)
			}
		})
	}
}

func TestNew_MissingProjectIsError(t *testing.T) {
	// We do not test against real Vertex AI here; the project guard
	// fires before the SDK is contacted, so this is a fast offline
	// check that the constructor refuses an obvious config bug.
	_, err := New(t.Context(), "", "us-central1", "gemini-2.5-flash", 0)
	if err == nil {
		t.Fatal("expected error for empty project, got nil")
	}
}
