package sessionbus

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUnknownSession = errors.New("session not found")
	ErrInvalidInput   = errors.New("invalid session bus input")
)

const (
	defaultSessionTTL      = 10 * time.Minute
	defaultMaxQueuePerSess = 128
	defaultMaxMessageBytes = 32 * 1024
	maxPollLimit           = 50
	maxPollWait            = 120 * time.Second
)

var validSessionID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$`)

type Config struct {
	SessionTTL      time.Duration
	MaxQueuePerSess int
	MaxMessageBytes int
}

type RegisterRequest struct {
	SessionID string
	Provider  string
	Name      string
}

type SendRequest struct {
	FromSessionID string
	ToSessionID   string
	Mode          string
	Message       string
}

type Session struct {
	SessionID  string    `json:"session_id"`
	Provider   string    `json:"provider"`
	Name       string    `json:"name,omitempty"`
	QueueDepth int       `json:"queue_depth"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type Envelope struct {
	ID            string    `json:"id"`
	FromSessionID string    `json:"from_session_id"`
	ToSessionID   string    `json:"to_session_id"`
	Mode          string    `json:"mode"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"created_at"`
}

type state struct {
	info   Session
	queue  []Envelope
	notify chan struct{}
}

type Bus struct {
	mu       sync.Mutex
	cfg      Config
	sessions map[string]*state
	nowFn    func() time.Time
}

func New(cfg Config) *Bus {
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = defaultSessionTTL
	}
	if cfg.MaxQueuePerSess <= 0 {
		cfg.MaxQueuePerSess = defaultMaxQueuePerSess
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = defaultMaxMessageBytes
	}

	return &Bus{
		cfg:      cfg,
		sessions: map[string]*state{},
		nowFn:    time.Now,
	}
}

func (b *Bus) Register(_ context.Context, req RegisterRequest) (Session, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	if !isValidSessionID(sessionID) {
		return Session{}, fmt.Errorf("%w: session_id must match %s", ErrInvalidInput, validSessionID.String())
	}

	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = "unknown"
	}
	name := strings.TrimSpace(req.Name)

	now := b.nowFn().UTC()

	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(now)

	st, ok := b.sessions[sessionID]
	if !ok {
		st = &state{notify: make(chan struct{})}
		b.sessions[sessionID] = st
	}
	st.info.SessionID = sessionID
	st.info.Provider = provider
	st.info.Name = name
	st.info.LastSeenAt = now
	st.info.QueueDepth = len(st.queue)

	return st.info, nil
}

func (b *Bus) List(_ context.Context) []Session {
	now := b.nowFn().UTC()

	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(now)

	out := make([]Session, 0, len(b.sessions))
	for _, st := range b.sessions {
		info := st.info
		info.QueueDepth = len(st.queue)
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SessionID < out[j].SessionID
	})
	return out
}

func (b *Bus) Send(_ context.Context, req SendRequest) (Envelope, error) {
	fromID := strings.TrimSpace(req.FromSessionID)
	toID := strings.TrimSpace(req.ToSessionID)
	mode := normalizeMode(req.Mode)
	msg := strings.TrimSpace(req.Message)

	if !isValidSessionID(fromID) {
		return Envelope{}, fmt.Errorf("%w: from_session_id is required", ErrInvalidInput)
	}
	if !isValidSessionID(toID) {
		return Envelope{}, fmt.Errorf("%w: to_session_id is required", ErrInvalidInput)
	}
	if fromID == toID {
		return Envelope{}, fmt.Errorf("%w: from_session_id and to_session_id must differ", ErrInvalidInput)
	}
	if msg == "" {
		return Envelope{}, fmt.Errorf("%w: message is required", ErrInvalidInput)
	}
	if len([]byte(msg)) > b.cfg.MaxMessageBytes {
		return Envelope{}, fmt.Errorf("%w: message exceeds max bytes (%d)", ErrInvalidInput, b.cfg.MaxMessageBytes)
	}

	now := b.nowFn().UTC()
	env := Envelope{
		ID:            uuid.New().String(),
		FromSessionID: fromID,
		ToSessionID:   toID,
		Mode:          mode,
		Message:       msg,
		CreatedAt:     now,
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(now)

	fromState := b.ensureSessionLocked(fromID, "unknown", "", now)
	toState, ok := b.sessions[toID]
	if !ok {
		return Envelope{}, ErrUnknownSession
	}

	fromState.info.LastSeenAt = now
	toState.info.LastSeenAt = now

	toState.queue = append(toState.queue, env)
	if extra := len(toState.queue) - b.cfg.MaxQueuePerSess; extra > 0 {
		toState.queue = toState.queue[extra:]
	}
	toState.info.QueueDepth = len(toState.queue)
	b.notifyLocked(toState)

	return env, nil
}

func (b *Bus) Poll(ctx context.Context, sessionID string, limit int, wait time.Duration) ([]Envelope, error) {
	sessionID = strings.TrimSpace(sessionID)
	if !isValidSessionID(sessionID) {
		return nil, fmt.Errorf("%w: session_id is required", ErrInvalidInput)
	}
	if limit <= 0 {
		limit = 1
	}
	if limit > maxPollLimit {
		limit = maxPollLimit
	}
	if wait < 0 {
		wait = 0
	}
	if wait > maxPollWait {
		wait = maxPollWait
	}

	start := b.nowFn()
	for {
		now := b.nowFn().UTC()

		b.mu.Lock()
		b.pruneLocked(now)

		st, ok := b.sessions[sessionID]
		if !ok {
			b.mu.Unlock()
			return nil, ErrUnknownSession
		}

		st.info.LastSeenAt = now
		if len(st.queue) > 0 {
			n := limit
			if len(st.queue) < n {
				n = len(st.queue)
			}
			out := append([]Envelope(nil), st.queue[:n]...)
			st.queue = st.queue[n:]
			st.info.QueueDepth = len(st.queue)
			b.mu.Unlock()
			return out, nil
		}

		if wait == 0 {
			b.mu.Unlock()
			return nil, nil
		}

		waitCh := st.notify
		remaining := wait - b.nowFn().Sub(start)
		b.mu.Unlock()

		if remaining <= 0 {
			return nil, nil
		}

		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-waitCh:
			if !timer.Stop() {
				<-timer.C
			}
			continue
		case <-timer.C:
			return nil, nil
		}
	}
}

func (b *Bus) ensureSessionLocked(sessionID, provider, name string, now time.Time) *state {
	st, ok := b.sessions[sessionID]
	if !ok {
		st = &state{notify: make(chan struct{})}
		b.sessions[sessionID] = st
	}
	if st.info.SessionID == "" {
		st.info.SessionID = sessionID
	}
	if strings.TrimSpace(provider) != "" {
		st.info.Provider = provider
	}
	if strings.TrimSpace(name) != "" {
		st.info.Name = name
	}
	if st.info.Provider == "" {
		st.info.Provider = "unknown"
	}
	st.info.LastSeenAt = now
	st.info.QueueDepth = len(st.queue)
	return st
}

func (b *Bus) notifyLocked(st *state) {
	close(st.notify)
	st.notify = make(chan struct{})
}

func (b *Bus) pruneLocked(now time.Time) {
	if b.cfg.SessionTTL <= 0 {
		return
	}
	for id, st := range b.sessions {
		if st.info.LastSeenAt.IsZero() {
			continue
		}
		if now.Sub(st.info.LastSeenAt) <= b.cfg.SessionTTL {
			continue
		}
		delete(b.sessions, id)
	}
}

func isValidSessionID(value string) bool {
	return validSessionID.MatchString(strings.TrimSpace(value))
}

func normalizeMode(value string) string {
	mode := strings.ToLower(strings.TrimSpace(value))
	switch mode {
	case "follow_up", "follow-up", "followup":
		return "follow_up"
	case "steer", "":
		return "steer"
	default:
		return "steer"
	}
}
