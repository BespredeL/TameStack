package ssh

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func noopExecHandler(ch ssh.Channel, cmd string) {}

// dialPoolClient builds a real in-process SSH client without registering test
// cleanup, so the pool exclusively owns the connection lifecycle.
func dialPoolClient(addr string) (*SSHClient, error) {
	cl, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &SSHClient{client: cl, host: "127.0.0.1", user: "root", stopKeepAlive: make(chan struct{})}, nil
}

func newTestPool(t *testing.T, addr string) *Pool {
	t.Helper()
	p := NewPool()
	p.dial = func(ctx context.Context, opts ClientOptions) (*SSHClient, error) {
		return dialPoolClient(addr)
	}
	t.Cleanup(p.CloseAll)
	return p
}

func poolClientCount(p *Pool) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.clients)
}

func TestPoolGetCachesLiveClient(t *testing.T) {
	addr := startTestSSHServer(t, noopExecHandler)
	p := newTestPool(t, addr)
	ctx := context.Background()
	opts := ClientOptions{Host: "127.0.0.1"}

	c1, err := p.Get(ctx, "s1", opts)
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	c2, err := p.Get(ctx, "s1", opts)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	if c1 != c2 {
		t.Fatal("expected the cached client to be reused")
	}
	if !c1.IsAlive() {
		t.Fatal("pooled client should be alive")
	}
	if n := poolClientCount(p); n != 1 {
		t.Fatalf("expected 1 pooled client, got %d", n)
	}
}

func TestPoolGetDialError(t *testing.T) {
	p := NewPool()
	t.Cleanup(p.CloseAll)
	sentinel := errors.New("dial failed")
	p.dial = func(ctx context.Context, opts ClientOptions) (*SSHClient, error) {
		return nil, sentinel
	}
	_, err := p.Get(context.Background(), "s1", ClientOptions{Host: "h"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped dial error, got %v", err)
	}
	if n := poolClientCount(p); n != 0 {
		t.Fatalf("failed dial should not pool a client, got %d", n)
	}
}

func TestPoolGetReplacesDeadClient(t *testing.T) {
	addr := startTestSSHServer(t, noopExecHandler)
	p := newTestPool(t, addr)
	ctx := context.Background()
	opts := ClientOptions{Host: "127.0.0.1"}

	c1, err := p.Get(ctx, "s1", opts)
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	_ = c1.Close() // simulate a dropped transport

	c2, err := p.Get(ctx, "s1", opts)
	if err != nil {
		t.Fatalf("get after death: %v", err)
	}
	if c2 == c1 {
		t.Fatal("expected a fresh client to replace the dead one")
	}
	if !c2.IsAlive() {
		t.Fatal("replacement client should be alive")
	}
}

func TestPoolInvalidateCloses(t *testing.T) {
	addr := startTestSSHServer(t, noopExecHandler)
	p := newTestPool(t, addr)
	c, err := p.Get(context.Background(), "s1", ClientOptions{Host: "127.0.0.1"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	p.Invalidate("s1")
	if c.IsAlive() {
		t.Fatal("client should be closed after invalidate")
	}
	if n := poolClientCount(p); n != 0 {
		t.Fatalf("expected empty pool after invalidate, got %d", n)
	}
}

func TestPoolCloseAll(t *testing.T) {
	addr := startTestSSHServer(t, noopExecHandler)
	p := newTestPool(t, addr)
	ctx := context.Background()
	opts := ClientOptions{Host: "127.0.0.1"}
	c1, err := p.Get(ctx, "s1", opts)
	if err != nil {
		t.Fatalf("get s1: %v", err)
	}
	c2, err := p.Get(ctx, "s2", opts)
	if err != nil {
		t.Fatalf("get s2: %v", err)
	}
	p.CloseAll()
	if c1.IsAlive() || c2.IsAlive() {
		t.Fatal("all clients should be closed after CloseAll")
	}
	if n := poolClientCount(p); n != 0 {
		t.Fatalf("expected empty pool after CloseAll, got %d", n)
	}
	select {
	case <-p.stop:
	default:
		t.Fatal("stop channel should be closed to terminate the reaper")
	}
}

func TestPoolEvictIdle(t *testing.T) {
	addr := startTestSSHServer(t, noopExecHandler)
	p := newTestPool(t, addr)
	ctx := context.Background()
	opts := ClientOptions{Host: "127.0.0.1"}
	stale, err := p.Get(ctx, "stale", opts)
	if err != nil {
		t.Fatalf("get stale: %v", err)
	}
	fresh, err := p.Get(ctx, "fresh", opts)
	if err != nil {
		t.Fatalf("get fresh: %v", err)
	}

	now := time.Now()
	p.mu.Lock()
	p.lastUsed["stale"] = now.Add(-p.idleTimeout - time.Minute)
	p.lastUsed["fresh"] = now
	p.mu.Unlock()

	p.evictIdle(now)

	if stale.IsAlive() {
		t.Fatal("idle client should be evicted and closed")
	}
	if !fresh.IsAlive() {
		t.Fatal("recently used client should survive eviction")
	}
	p.mu.Lock()
	_, staleOK := p.clients["stale"]
	_, freshOK := p.clients["fresh"]
	p.mu.Unlock()
	if staleOK {
		t.Fatal("stale entry should be removed from the pool")
	}
	if !freshOK {
		t.Fatal("fresh entry should remain in the pool")
	}
}

func TestPoolConcurrentGetConverges(t *testing.T) {
	addr := startTestSSHServer(t, noopExecHandler)
	p := newTestPool(t, addr)

	const n = 8
	var wg sync.WaitGroup
	clients := make([]*SSHClient, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			clients[i], errs[i] = p.Get(context.Background(), "shared", ClientOptions{Host: "127.0.0.1"})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent get %d: %v", i, err)
		}
	}
	for i := 1; i < n; i++ {
		if clients[i] != clients[0] {
			t.Fatal("concurrent gets should converge on a single pooled client")
		}
	}
	if got := poolClientCount(p); got != 1 {
		t.Fatalf("expected 1 pooled client after concurrent gets, got %d", got)
	}
}
