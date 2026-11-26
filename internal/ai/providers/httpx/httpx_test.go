package httpx

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewClient_SetsResponseHeaderTimeout(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if tr.ResponseHeaderTimeout != 30*time.Second {
		t.Errorf("ResponseHeaderTimeout = %v, want 30s", tr.ResponseHeaderTimeout)
	}
	if client.Timeout != 0 {
		t.Errorf("client.Timeout = %v, want 0 (streaming bounded by context)", client.Timeout)
	}
}

func TestStatusError_CapsBodyRead(t *testing.T) {
	oversized := bytes.Repeat([]byte("a"), MaxErrorBodyBytes+1024)
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(bytes.NewReader(oversized)),
	}
	err := StatusError(resp, "openai chat")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "openai chat returned status 502: ") {
		t.Errorf("unexpected error prefix: %q", msg[:min(len(msg), 60)])
	}
	if len(msg) > MaxErrorBodyBytes+100 {
		t.Errorf("error message length %d exceeds capped bound", len(msg))
	}
}
