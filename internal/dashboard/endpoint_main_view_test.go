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

func TestPerformancePercentileFromSortedValues(t *testing.T) {
	values := []float64{10, 20, 30, 40}

	if got := performancePercentileFromSortedValues(values, 40, false); got != 100 {
		t.Fatalf("expected highest higher-is-better percentile to be 100, got %.2f", got)
	}
	if got := performancePercentileFromSortedValues(values, 10, false); got != 0 {
		t.Fatalf("expected lowest higher-is-better percentile to be 0, got %.2f", got)
	}
	if got := performancePercentileFromSortedValues(values, 10, true); got != 100 {
		t.Fatalf("expected lowest lower-is-better percentile to be 100, got %.2f", got)
	}
	if got := performancePercentileFromSortedValues(values, 40, true); got != 0 {
		t.Fatalf("expected highest lower-is-better percentile to be 0, got %.2f", got)
	}
}
