package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// startTestSSHServer runs an in-process SSH server whose exec handler is supplied
// by the test. It returns the listen address; everything is cleaned up via t.Cleanup.
func startTestSSHServer(t *testing.T, handler func(ch ssh.Channel, cmd string)) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				sconn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer sconn.Close()
				go ssh.DiscardRequests(reqs)
				for newCh := range chans {
					if newCh.ChannelType() != "session" {
						_ = newCh.Reject(ssh.UnknownChannelType, "unsupported")
						continue
					}
					ch, chReqs, err := newCh.Accept()
					if err != nil {
						continue
					}
					go func() {
						defer ch.Close()
						for req := range chReqs {
							if req.Type != "exec" {
								_ = req.Reply(false, nil)
								continue
							}
							var payload struct{ Command string }
							if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
								_ = req.Reply(false, nil)
								continue
							}
							_ = req.Reply(true, nil)
							handler(ch, payload.Command)
							_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
							return
						}
					}()
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func dialTestClient(t *testing.T, addr string) *SSHClient {
	t.Helper()
	cl, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &SSHClient{client: cl, host: "127.0.0.1", user: "root", stopKeepAlive: make(chan struct{})}
	t.Cleanup(func() { c.Close() })
	return c
}

func floodHandler(ch ssh.Channel, cmd string) {
	line := []byte("flood line with some payload to keep the writer busy\n")
	for {
		if _, err := ch.Write(line); err != nil {
			return
		}
	}
}

func TestExecRawCancelWaitsForWriter(t *testing.T) {
	addr := startTestSSHServer(t, floodHandler)
	c := dialTestClient(t, addr)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	var exitCode int
	var err error
	go func() {
		_, _, exitCode, err = c.execRaw(ctx, "flood", nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("execRaw did not return after context cancellation")
	}
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if exitCode != -1 {
		t.Fatalf("expected exit code -1 on cancel, got %d", exitCode)
	}
}

func TestExecStreamCancelStopsCallbacks(t *testing.T) {
	addr := startTestSSHServer(t, floodHandler)
	c := dialTestClient(t, addr)

	var calls atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	var err error
	go func() {
		_, err = c.ExecStream(ctx, "flood", func(string) { calls.Add(1) }, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("ExecStream did not return after context cancellation")
	}
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	after := calls.Load()
	time.Sleep(150 * time.Millisecond)
	if got := calls.Load(); got != after {
		t.Fatalf("stdout callbacks fired after ExecStream returned: %d -> %d", after, got)
	}
}

func TestExecRawExitStatusMapping(t *testing.T) {
	addr := startTestSSHServer(t, func(ch ssh.Channel, cmd string) {
		fmt.Fprint(ch, "hello")
	})
	c := dialTestClient(t, addr)

	stdout, _, exitCode, err := c.execRaw(context.Background(), "echo", nil)
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if exitCode != 0 || string(stdout) != "hello" {
		t.Fatalf("unexpected result: exit=%d stdout=%q", exitCode, stdout)
	}
}
