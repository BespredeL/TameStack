package router

import (
	"strings"
	"unicode/utf8"

	"tamestack/internal/ai/domain"
)

// EstimateTokens calculates an approximate token count for a text string.
// Handles mixed Latin, Cyrillic, and code text reasonably without full BPE dependencies.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	runeCount := utf8.RuneCountInString(text)
	words := len(strings.Fields(text))

	// For Cyrillic and multi-byte scripts, average ~1.5 to 2 tokens per word or ~2.5 runes per token.
	// For English and code, ~4 chars per token.
	estimated := (runeCount + words*3) / 3
	if estimated < 1 {
		estimated = 1
	}
	return estimated
}

// EstimateChatCost calculates expected cost in USD for a given prompt and target model.
func EstimateChatCost(spec domain.ModelSpec, promptTokens, expectedCompletionTokens int) float64 {
	if spec.IsLocal || (spec.InputCostPerM == 0 && spec.OutputCostPerM == 0) {
		return 0.0
	}

	inCost := (float64(promptTokens) / 1000000.0) * spec.InputCostPerM
	outCost := (float64(expectedCompletionTokens) / 1000000.0) * spec.OutputCostPerM
	return inCost + outCost
}
