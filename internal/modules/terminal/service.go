package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"tamestack/internal/config"
	"tamestack/internal/crypto"
	"tamestack/internal/db"
	"tamestack/internal/module"
	"tamestack/internal/ssh"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	cryptoSSH "golang.org/x/crypto/ssh"
)

type activeSession struct {
	sessionID string
	serverID  string
	sshSess   *cryptoSSH.Session
	stdin     io.WriteCloser
	cancel    context.CancelFunc
}

type Service struct {
	mu sync.Mutex
	module.Base
	appCtx   context.Context
	sessions map[string]*activeSession
}

func NewService(cfg *config.AppConfig, database *db.DB, vault *crypto.Vault, pool *ssh.Pool) *Service {
	return &Service{
		Base:     module.Base{Cfg: cfg, Database: database, Vault: vault, Pool: pool},
		sessions: make(map[string]*activeSession),
	}
}

// SetContext stores the Wails context used to emit events.
func (s *Service) SetContext(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appCtx = ctx
}

// StartSession opens an interactive SSH PTY session.
func (s *Service) StartSession(ctx context.Context, req StartSessionRequest) (*TerminalSessionInfo, error) {
	client, _, err := s.GetSSHClient(ctx, req.ServerID)
	if err != nil {
		return nil, err
	}

	sshSess, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("не удалось создать SSH сессию: %w", err)
	}

	cols := req.Cols
	if cols <= 0 {
		cols = 100
	}
	rows := req.Rows
	if rows <= 0 {
		rows = 30
	}

	modes := cryptoSSH.TerminalModes{
		cryptoSSH.ECHO:          1,
		cryptoSSH.TTY_OP_ISPEED: 14400,
		cryptoSSH.TTY_OP_OSPEED: 14400,
	}

	if err := sshSess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = sshSess.Close()
		return nil, fmt.Errorf("ошибка запроса PTY: %w", err)
	}

	stdin, err := sshSess.StdinPipe()
	if err != nil {
		_ = sshSess.Close()
		return nil, fmt.Errorf("ошибка создания StdinPipe: %w", err)
	}

	stdout, err := sshSess.StdoutPipe()
	if err != nil {
		_ = sshSess.Close()
		return nil, fmt.Errorf("ошибка создания StdoutPipe: %w", err)
	}

	stderr, err := sshSess.StderrPipe()
	if err != nil {
		_ = sshSess.Close()
		return nil, fmt.Errorf("ошибка создания StderrPipe: %w", err)
	}

	if err := sshSess.Shell(); err != nil {
		_ = sshSess.Close()
		return nil, fmt.Errorf("ошибка запуска Shell: %w", err)
	}

	sessCtx, sessCancel := context.WithCancel(context.Background())
	sessionID := uuid.New().String()

	sessObj := &activeSession{
		sessionID: sessionID,
		serverID:  req.ServerID,
		sshSess:   sshSess,
		stdin:     stdin,
		cancel:    sessCancel,
	}

	s.mu.Lock()
	s.sessions[sessionID] = sessObj
	appCtx := s.appCtx
	s.mu.Unlock()

	// Goroutine reading stdout.
	go func() {
		pumpUTF8Stream(sessCtx, stdout, func(chunk string) {
			if appCtx != nil {
				wailsRuntime.EventsEmit(appCtx, "terminal:data:"+sessionID, chunk)
			}
		})
		if appCtx != nil {
			wailsRuntime.EventsEmit(appCtx, "terminal:exit:"+sessionID, true)
		}
		_ = s.CloseSession(sessionID)
	}()

	// Goroutine reading stderr.
	go func() {
		pumpUTF8Stream(sessCtx, stderr, func(chunk string) {
			if appCtx != nil {
				wailsRuntime.EventsEmit(appCtx, "terminal:data:"+sessionID, chunk)
			}
		})
	}()

	// Subscribe to input events from the GUI.
	if appCtx != nil {
		wailsRuntime.EventsOn(appCtx, "terminal:input:"+sessionID, func(data ...interface{}) {
			if len(data) > 0 {
				if str, ok := data[0].(string); ok {
					_ = s.WriteInput(sessionID, str)
				}
			}
		})

		wailsRuntime.EventsOn(appCtx, "terminal:resize:"+sessionID, func(data ...interface{}) {
			if len(data) >= 2 {
				c, ok1 := data[0].(float64)
				r, ok2 := data[1].(float64)
				if ok1 && ok2 {
					_ = s.Resize(sessionID, int(c), int(r))
				}
			}
		})
	}

	return &TerminalSessionInfo{
		SessionID: sessionID,
		ServerID:  req.ServerID,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// WriteInput sends characters or commands to the terminal's stdin.
func (s *Service) WriteInput(sessionID string, data string) error {
	s.mu.Lock()
	sess, exists := s.sessions[sessionID]
	s.mu.Unlock()

	if !exists || sess.stdin == nil {
		return errors.New("терминальная сессия не найдена или закрыта")
	}

	_, err := sess.stdin.Write([]byte(data))
	return err
}

// Resize sends a WindowChange signal to resize the PTY rows and columns.
func (s *Service) Resize(sessionID string, cols int, rows int) error {
	s.mu.Lock()
	sess, exists := s.sessions[sessionID]
	s.mu.Unlock()

	if !exists || sess.sshSess == nil {
		return errors.New("сессия не найдена")
	}

	return sess.sshSess.WindowChange(rows, cols)
}

// CloseSession cleanly terminates an interactive session.
func (s *Service) CloseSession(sessionID string) error {
	s.mu.Lock()
	sess, exists := s.sessions[sessionID]
	if exists {
		delete(s.sessions, sessionID)
	}
	appCtx := s.appCtx
	s.mu.Unlock()

	if !exists {
		return nil
	}

	sess.cancel()

	if sess.stdin != nil {
		_ = sess.stdin.Close()
	}
	if sess.sshSess != nil {
		_ = sess.sshSess.Close()
	}

	if appCtx != nil {
		wailsRuntime.EventsOff(appCtx, "terminal:input:"+sessionID)
		wailsRuntime.EventsOff(appCtx, "terminal:resize:"+sessionID)
	}

	return nil
}

// pumpUTF8Stream reads from r and invokes emit with valid UTF-8 boundary-aligned chunks,
// holding back any incomplete trailing multi-byte rune until the next read.
func pumpUTF8Stream(ctx context.Context, r io.Reader, emit func(string)) {
	buf := make([]byte, 4096)
	var pending []byte
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := r.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			cut := trailingIncompleteUTF8(pending)
			if cut > 0 {
				emit(string(pending[:cut]))
				pending = append(pending[:0], pending[cut:]...)
			}
		}
		if err != nil {
			if len(pending) > 0 && ctx.Err() == nil {
				emit(string(pending))
			}
			return
		}
	}
}

// trailingIncompleteUTF8 returns the index up to which data ends on a complete UTF-8 rune boundary.
func trailingIncompleteUTF8(data []byte) int {
	n := len(data)
	// A UTF-8 rune is at most 4 bytes; inspect up to the last 3 bytes for an incomplete leading sequence.
	for i := 1; i <= 3 && i <= n; i++ {
		b := data[n-i]
		if b < utf8.RuneSelf {
			return n
		}
		if utf8.RuneStart(b) {
			if utf8.FullRune(data[n-i:]) {
				return n
			}
			return n - i
		}
	}
	return n
}
