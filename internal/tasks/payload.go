package tasks

import (
	"encoding/json"
	"fmt"
)

// MarshalPayload encodes a typed task payload shared by submit site and handler.
func MarshalPayload(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to encode task payload: %w", err)
	}
	return string(b), nil
}

// DecodePayload decodes a task payload into the typed struct the handler expects.
func DecodePayload[T any](payload string) (T, error) {
	var p T
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return p, fmt.Errorf("invalid task payload: %w", err)
	}
	return p, nil
}
