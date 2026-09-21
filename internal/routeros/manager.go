package routeros

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func newToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// Session is one logical RouterosAPI instance handed out to a PHP request.
type Session struct {
	ID       string
	pool     *Pool
	created  time.Time
	lastSeen time.Time
}

// Manager owns every pool and session. It is safe for concurrent use.
type Manager struct {
	maxConn     int
	idleTimeout time.Duration
	closeIdle   time.Duration

	mu       sync.Mutex
	pools    map[string]*Pool
	sessions map[string]*Session
	stop     chan struct{}
	stopped  bool
}

// ManagerOptions configures a Manager.
type ManagerOptions struct {
	// MaxConnPerRouter bounds concurrent API connections to a single router.
	MaxConnPerRouter int
	// IdleTimeout drops pooled connections unused for this long.
	IdleTimeout time.Duration
	// SessionTTL expires sessions not touched for this long.
	SessionTTL time.Duration
	// ReapInterval is the sweeper period.
	ReapInterval time.Duration
}

// NewManager creates a Manager and starts its background reaper.
func NewManager(opt ManagerOptions) *Manager {
	if opt.MaxConnPerRouter < 1 {
		opt.MaxConnPerRouter = 32
	}
	if opt.IdleTimeout <= 0 {
		opt.IdleTimeout = 60 * time.Second
	}
	if opt.SessionTTL <= 0 {
		opt.SessionTTL = 10 * time.Minute
	}
	if opt.ReapInterval <= 0 {
		opt.ReapInterval = 15 * time.Second
	}

	m := &Manager{
		maxConn:     opt.MaxConnPerRouter,
		idleTimeout: opt.IdleTimeout,
		closeIdle:   opt.IdleTimeout,
		pools:       make(map[string]*Pool),
		sessions:    make(map[string]*Session),
		stop:        make(chan struct{}),
	}
	go m.reapLoop(opt.ReapInterval, opt.SessionTTL)
	return m
}

// Close shuts every pool down.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.stop)
	pools := make([]*Pool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, p)
	}
	m.pools = map[string]*Pool{}
	m.sessions = map[string]*Session{}
	m.mu.Unlock()

	for _, p := range pools {
		p.shutdown()
	}
}

// Connect validates credentials against the router and opens a session.
//
// The TCP connection is kept in the pool so subsequent commands issued by the
// same PHP request (and later requests) reuse it instead of dialling again.
func (m *Manager) Connect(t Target) (*Session, error) {
	if t.Addr == "" {
		return nil, errors.New("routeros: missing host")
	}

	key := t.Key()

	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return nil, ErrPoolClosed
	}
	p, ok := m.pools[key]
	if !ok {
		p = newPool(key, t, m.maxConn)
		m.pools[key] = p
	}
	m.mu.Unlock()

	// Warm the pool: this is what actually proves the credentials work, so the
	// PHP side can faithfully set $API->connected.
	c, err := p.acquire()
	if err != nil {
		return nil, err
	}
	p.release(c)

	s := &Session{
		ID:       newToken(),
		pool:     p,
		created:  time.Now(),
		lastSeen: time.Now(),
	}

	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()

	return s, nil
}

// Exec runs a command on the pool behind a session.
func (m *Manager) Exec(sessionID string, timeout time.Duration, words []string) ([]Sentence, error) {
	s, err := m.Session(sessionID)
	if err != nil {
		return nil, err
	}
	return s.pool.Exec(timeout, words)
}

// Session looks a session up and refreshes its idle timer.
func (m *Manager) Session(id string) (*Session, error) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		s.lastSeen = time.Now()
	}
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("routeros: unknown session %q", id)
	}
	return s, nil
}

// PoolFor returns the pool behind a session, for callers that need several
// connections at once (bulk operations).
func (m *Manager) PoolFor(sessionID string) (*Pool, error) {
	s, err := m.Session(sessionID)
	if err != nil {
		return nil, err
	}
	return s.pool, nil
}

// ExecBatch runs independent commands concurrently across the pool.
// The result slice is index-aligned with cmds; a per-command error never
// aborts the others.
func (m *Manager) ExecBatch(sessionID string, concurrency int, timeout time.Duration, cmds [][]string) ([]error, error) {
	pool, err := m.PoolFor(sessionID)
	if err != nil {
		return nil, err
	}
	if concurrency < 1 {
		concurrency = 1
	}

	errs := make([]error, len(cmds))
	if len(cmds) == 0 {
		return errs, nil
	}

	idx := make(chan int)
	var wg sync.WaitGroup

	workers := concurrency
	if workers > len(cmds) {
		workers = len(cmds)
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				c, aerr := pool.acquire()
				if aerr != nil {
					errs[i] = aerr
					continue
				}
				if timeout > 0 {
					c.SetTimeout(timeout)
				}
				_, rerr := c.RunCommand(cmds[i]...)
				if rerr != nil {
					pool.discard(c)
					errs[i] = rerr
					continue
				}
				pool.release(c)
			}
		}()
	}

	for i := range cmds {
		idx <- i
	}
	close(idx)
	wg.Wait()

	return errs, nil
}

// Stats reports pool counters, for the /healthz endpoint.
func (m *Manager) Stats() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	routers := make(map[string]any, len(m.pools))
	for k, p := range m.pools {
		open, idle := p.stats()
		routers[hashKeyLabel(k)] = map[string]int{"open": open, "idle": idle}
	}
	return map[string]any{
		"routers":  routers,
		"sessions": len(m.sessions),
	}
}

func (m *Manager) reapLoop(interval, sessionTTL time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			m.reap(sessionTTL)
		}
	}
}

func (m *Manager) reap(sessionTTL time.Duration) {
	now := time.Now()

	m.mu.Lock()
	for id, s := range m.sessions {
		if now.Sub(s.lastSeen) > sessionTTL {
			delete(m.sessions, id)
		}
	}
	var dead []*Pool
	for k, p := range m.pools {
		if d, free := p.idleSince(); free && d > m.idleTimeout*2 {
			dead = append(dead, p)
			delete(m.pools, k)
		}
	}
	m.mu.Unlock()

	for _, p := range dead {
		p.shutdown()
	}
}

// hashKeyLabel strips credentials out of a pool key for display.
func hashKeyLabel(key string) string {
	parts := strings.SplitN(key, "|", 3)
	if len(parts) >= 3 {
		return parts[0] + "|" + parts[1]
	}
	return key
}
