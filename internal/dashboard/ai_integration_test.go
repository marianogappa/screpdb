package dashboard

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestGeminiIntegration_LLMConstruction(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping Gemini integration test")
	}

	ctx := context.Background()
	_, err := newGeminiLLM(apiKey, "")
	if err != nil {
		t.Fatalf("failed to create Gemini LLM: %v", err)
	}
	t.Logf("Gemini LLM created successfully with model %s", GeminiDefaultModel)

	_ = ctx
}

func newGeminiLLM(apiKey, model string) (*AI, error) {
	if model == "" {
		model = GeminiDefaultModel
	}
	a := &AI{ctx: context.Background()}
	opt := WithGemini(apiKey, model)
	if err := opt(a); err != nil {
		return nil, fmt.Errorf("WithGemini: %w", err)
	}
	return a, nil
}
