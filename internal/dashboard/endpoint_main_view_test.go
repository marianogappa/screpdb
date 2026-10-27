package dashboard

import (
	"slices"
	"testing"
)

func TestSummarizeChatTokensKeepsGG(t *testing.T) {
	tokens := summarizeChatTokens("gg go scout now")
	if !slices.Contains(tokens, "gg") {
		t.Fatalf("expected gg token to be preserved, got %v", tokens)
	}
	if slices.Contains(tokens, "go") {
		t.Fatalf("expected short filler token to be removed, got %v", tokens)
	}
}
