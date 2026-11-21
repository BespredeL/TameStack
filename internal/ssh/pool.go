package ssh

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Pool manages persistent SSH connections for servers to minimize handshake overhead.
type Pool struct {
	mu          sync.Mutex
	clients     map[string]*SSHClient
	lastUsed    map[string]time.Time
	idleTimeout time.Duration
	stop        chan struct{}
	dial        func(context.Context, ClientOptions) (*SSHClient, error)
}

// NewPool initializes a thread-safe connection pool.
func NewPool() *Pool {
	p := &Pool{
		clients:     make(map[string]*SSHClient),
		lastUsed:    make(map[string]time.Time),
		idleTimeout: 10 * time.Minute,
		stop:        make(chan struct{}),
		dial:        Connect,
	}
	go p.reapIdle()
	return p
}

// Get retrieves an existing active client or dials a new one.
// The pool lock is not held during the TCP/SSH handshake.
func (p *Pool) Get(ctx context.Context, serverID string, opts ClientOptions) (*SSHClient, error) {
	p.mu.Lock()
	existing := p.clients[serverID]
	p.mu.Unlock()

	if existing != nil {
		if existing.IsAlive() {
			p.applySudo(existing, opts)
			p.touch(serverID)
			return existing, nil
		}
		p.Invalidate(serverID)
	}

	client, err := p.dial(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server %s: %w", opts.Host, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if current, exists := p.clients[serverID]; exists && current.IsAlive() {
		_ = client.Close()
		p.applySudoLocked(current, opts)
		p.lastUsed[serverID] = time.Now()
		return current, nil
	}
	if old, exists := p.clients[serverID]; exists {
		_ = old.Close()
	}
	p.clients[serverID] = client
	p.lastUsed[serverID] = time.Now()
	return client, nil
}

func (p *Pool) applySudo(client *SSHClient, opts ClientOptions) {
	if opts.SudoPassword != "" {
		client.SetSudoPassword(opts.SudoPassword)
	} else if client.GetSudoPassword() == "" && opts.AuthType == "password" {
		client.SetSudoPassword(opts.Secret)
	}
}

func (p *Pool) applySudoLocked(client *SSHClient, opts ClientOptions) {
	p.applySudo(client, opts)
}

func (p *Pool) touch(serverID string) {
	p.mu.Lock()
	p.lastUsed[serverID] = time.Now()
	p.mu.Unlock()
}

// Invalidate closes and removes a connection for a specific server (e.g. after credential change).
func (p *Pool) Invalidate(serverID string) {
	p.mu.Lock()
	client, exists := p.clients[serverID]
	if exists {
		delete(p.clients, serverID)
		delete(p.lastUsed, serverID)
	}
	p.mu.Unlock()
	if exists {
		_ = client.Close()
	}
}

// CloseAll closes all active connections in the pool.
func (p *Pool) CloseAll() {
	p.mu.Lock()
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	clients := p.clients
	p.clients = make(map[string]*SSHClient)
	p.lastUsed = make(map[string]time.Time)
	p.mu.Unlock()

	for _, client := range clients {
		_ = client.Close()
	}
}

func (p *Pool) reapIdle() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.evictIdle(time.Now())
		}
	}
}

// evictIdle closes and removes connections whose last use is older than
// idleTimeout relative to now. Split out from reapIdle so eviction can be
// exercised with a controlled timestamp.
func (p *Pool) evictIdle(now time.Time) {
	p.mu.Lock()
	var closing []*SSHClient
	for id, used := range p.lastUsed {
		if now.Sub(used) < p.idleTimeout {
			continue
		}
		if client, ok := p.clients[id]; ok {
			closing = append(closing, client)
		}
		delete(p.clients, id)
		delete(p.lastUsed, id)
	}
	p.mu.Unlock()
	for _, client := range closing {
		_ = client.Close()
	}
}
