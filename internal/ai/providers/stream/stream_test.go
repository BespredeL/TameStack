package stream

import (
	"context"
	"strings"
	"testing"
	"time"

	"tamestack/internal/ai/domain"
)

func TestRecoverEmitsErrorChunkBeforeClose(t *testing.T) {
	out := make(chan domain.StreamChunk, 1)

	func() {
		defer close(out)
		defer Recover(out, "testprovider")
		panic("stream broke")
	}()

	chunk, ok := <-out
	if !ok {
		t.Fatal("channel closed without an error chunk")
	}
	if chunk.Error == nil || !strings.Contains(chunk.Error.Error(), "stream broke") {
		t.Fatalf("panic not reported to the consumer: %+v", chunk)
	}
	if _, stillOpen := <-out; stillOpen {
		t.Fatal("channel must be closed after the goroutine unwinds")
	}
}

func TestSendGivesUpWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Unbuffered channel with no reader: a plain send would park forever.
	if Send(ctx, make(chan domain.StreamChunk), domain.StreamChunk{ContentDelta: "x"}) {
		t.Fatal("Send reported delivery with a cancelled context and no reader")
	}
}

func TestTrySendDropsWhenBufferFull(t *testing.T) {
	if TrySend(make(chan domain.StreamChunk), domain.StreamChunk{ContentDelta: "x"}) {
		t.Fatal("TrySend must not deliver to a full buffer")
	}
}

// The streaming goroutines use Send for every chunk; if the consumer walks away
// and the context is cancelled, the goroutine must still finish and release the
// response body it holds.
func TestProducerExitsAfterConsumerStopsReading(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan domain.StreamChunk)
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer close(out)
		for i := 0; i < 1000; i++ {
			if !Send(ctx, out, domain.StreamChunk{ContentDelta: "chunk"}) {
				return
			}
		}
	}()

	<-out
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("producer goroutine is still parked after the consumer stopped reading")
	}
}
