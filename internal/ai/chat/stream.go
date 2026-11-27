package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"tamestack/internal/ai/domain"
)

// streamResult holds everything accumulated from a single provider stream.
type streamResult struct {
	content          string
	promptTokens     int
	completionTokens int
	toolCalls        []domain.ToolCall
}

// accumulateStream drains ch, forwarding each non-error chunk to onChunk and
// accumulating content, token usage and tool-call deltas. On the first error
// chunk it stops and returns that chunk (without forwarding it) so the caller
// can decide whether to surface or swallow the error.
func accumulateStream(ch <-chan domain.StreamChunk, onChunk func(domain.StreamChunk)) (streamResult, *domain.StreamChunk) {
	var res streamResult
	for chunk := range ch {
		if chunk.Error != nil {
			return res, &chunk
		}
		res.content += chunk.ContentDelta
		res.toolCalls = appendToolCallDelta(res.toolCalls, chunk.ToolCallDelta)
		if chunk.Usage != nil {
			res.promptTokens = chunk.Usage.PromptTokens
			res.completionTokens = chunk.Usage.CompletionTokens
		}
		if onChunk != nil {
			onChunk(chunk)
		}
	}
	return res, nil
}

// appendToolCallDelta merges a streamed tool-call delta into calls, matching by
// ID when present and otherwise appending argument fragments to the last call.
func appendToolCallDelta(calls []domain.ToolCall, tc *domain.ToolCall) []domain.ToolCall {
	if tc == nil {
		return calls
	}
	if tc.Function.Name != "" {
		if tc.ID != "" {
			for i := range calls {
				if calls[i].ID == tc.ID {
					if tc.Function.Arguments != "" {
						calls[i].Function.Arguments += tc.Function.Arguments
					}
					return calls
				}
			}
		}
		return append(calls, *tc)
	}
	if len(calls) > 0 && tc.Function.Arguments != "" {
		calls[len(calls)-1].Function.Arguments += tc.Function.Arguments
	}
	return calls
}

// normalizeToolCalls drops calls with empty names, defaults empty arguments
// to "{}", and assigns a deterministic ID when providers omit tool call IDs.
func normalizeToolCalls(calls []domain.ToolCall) []domain.ToolCall {
	var valid []domain.ToolCall
	for i, tc := range calls {
		if strings.TrimSpace(tc.Function.Name) != "" {
			if strings.TrimSpace(tc.ID) == "" {
				tc.ID = fmt.Sprintf("call_%d", i+1)
			}
			if strings.TrimSpace(tc.Function.Arguments) == "" {
				tc.Function.Arguments = "{}"
			}
			valid = append(valid, tc)
		}
	}
	return valid
}

// marshalToolCalls serializes calls for persistence, returning "[]" when empty.
func marshalToolCalls(calls []domain.ToolCall) string {
	if len(calls) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(calls)
	return string(b)
}
