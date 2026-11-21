// Package stream holds the channel plumbing shared by the AI provider streaming
// goroutines.
package stream

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"tamestack/internal/ai/domain"
)

// Send forwards a chunk to the consumer and reports whether it was delivered.
// A plain blocking send would park the goroutine — and the HTTP response body it
// holds — forever once the consumer stops reading.
func Send(ctx context.Context, out chan<- domain.StreamChunk, chunk domain.StreamChunk) bool {
	select {
	case out <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}

// TrySend delivers a chunk only if the buffer has room. Use it on paths where the
// context is already cancelled or the goroutine is unwinding, so reporting the
// failure can never block the shutdown.
func TrySend(out chan<- domain.StreamChunk, chunk domain.StreamChunk) bool {
	select {
	case out <- chunk:
		return true
	default:
		return false
	}
}

// Recover turns a panic in a streaming goroutine into an error chunk. Register it
// after the defer that closes the channel so it runs while the channel is open.
func Recover(out chan<- domain.StreamChunk, provider string) {
	p := recover()
	if p == nil {
		return
	}
	slog.Error("provider stream goroutine panicked",
		slog.String("provider", provider),
		slog.Any("panic", p),
		slog.String("stack", string(debug.Stack())))
	TrySend(out, domain.StreamChunk{Error: fmt.Errorf("%s stream interrupted: %v", provider, p)})
}
