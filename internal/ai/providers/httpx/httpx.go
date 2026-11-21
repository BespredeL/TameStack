// Package httpx holds the HTTP client construction shared by the AI provider
// clients.
package httpx

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// MaxErrorBodyBytes caps how much of an error response is read into memory.
const MaxErrorBodyBytes = 1 << 20

// StatusError builds an error from a non-OK HTTP response, reading a capped
// portion of the body for diagnostics. label names the operation, so
// StatusError(resp, "openai chat") yields "openai chat returned status 500: ...".
func StatusError(resp *http.Response, label string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, MaxErrorBodyBytes))
	return fmt.Errorf("%s returned status %d: %s", label, resp.StatusCode, string(body))
}

// responseHeaderTimeout bounds the wait for response headers. Streaming replies
// then run as long as the server keeps sending, without a whole-request timeout
// cutting a healthy generation short.
const responseHeaderTimeout = 30 * time.Second

// NewClient returns a client with no whole-request timeout: streaming generations
// are bounded by the per-request context, not by a fixed deadline.
func NewClient() *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{}
	}
	cloned := transport.Clone()
	cloned.ResponseHeaderTimeout = responseHeaderTimeout
	return &http.Client{Transport: cloned}
}
