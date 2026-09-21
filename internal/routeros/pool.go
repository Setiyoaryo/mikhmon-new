package routeros

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Target describes how to reach one RouterOS device.
type Target struct {
	Addr     string        `json:"addr"`
	Username string        `json:"username"`
	Password string        `json:"password"`
	UseSSL   bool          `json:"ssl"`
	Timeout  time.Duration `json:"-"`

	// TimeoutMS is the JSON-friendly form of Timeout.
	TimeoutMS int64 `json:"timeout_ms,omitempty"`
}

// Key identifies a pool. Credentials are part of the key so that changing the
// password cannot reuse a connection authenticated with the old one.
func (t Target) Key() string {
	return fmt.Sprintf("%s|%s|%t|%s", t.Addr, t.Username, t.UseSSL, hashSecret(t.Password))
}

func (t Target) effectiveTimeout() time.Duration {
	if t.Timeout > 0 {
		return t.Timeout
	}
	if t.TimeoutMS > 0 {
		return time.Duration(t.TimeoutMS) * time.Millisecond
	}
	return 10 * time.Second
}

// ErrPoolClosed is returned when a pool has been shut down.
var ErrPoolClosed = errors.New("routeros: pool closed")

// isConnError reports whether err looks like a dead/closed connection rather
// than a router-side rejection (which arrives as "!trap" with a nil-ish error).
func isConnError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"closed", "reset", "broken pipe", "eof", "timeout", "refused",
		"no route", "unreachable", "use of closed", "i/o timeout",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// Pool is a bounded set of authenticated connections to a single router.
//
// Connections are created lazily, reused across requests, and dropped on error.
type Pool struct {
	key     string
	target  Target
	maxConn int

	mu       sync.Mutex
	cond     *sync.Cond
	idle     []*Client
	open     int
	closed   bool
	lastUsed time.Time
}

func newPool(key string, target Target, maxConn int) *Pool {
	if maxConn < 1 {
		maxConn = 1
	}
	p := &Pool{
		key:      key,
		target:   target,
		maxConn:  maxConn,
		lastUsed: time.Now(),
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// acquire returns an exclusive connection, dialling a new one when the pool has
// spare capacity, or blocking until another caller releases one.
func (p *Pool) acquire() (*Client, error) {
	p.mu.Lock()
	for {
		if p.closed {
			p.mu.Unlock()
			return nil, ErrPoolClosed
		}
		if n := len(p.idle); n > 0 {
			c := p.idle[n-1]
			p.idle = p.idle[:n-1]
			p.lastUsed = time.Now()
			p.mu.Unlock()
			return c, nil
		}
		if p.open < p.maxConn {
			p.open++
			p.mu.Unlock()

			t := p.target
			c, err := DialAndLogin(t.Addr, t.Username, t.Password, t.UseSSL, t.effectiveTimeout())

			p.mu.Lock()
			if err != nil {
				p.open--
				p.cond.Broadcast()
				p.mu.Unlock()
				return nil, err
			}
			if p.closed {
				p.open--
				p.mu.Unlock()
				_ = c.Close()
				return nil, ErrPoolClosed
			}
			p.mu.Unlock()
			return c, nil
		}
		p.cond.Wait()
	}
}

// release returns a healthy connection to the idle set.
func (p *Pool) release(c *Client) {
	if c == nil {
		return
	}
	p.mu.Lock()
	if p.closed {
		p.open--
		p.mu.Unlock()
		_ = c.Close()
		return
	}
	p.idle = append(p.idle, c)
	p.lastUsed = time.Now()
	p.mu.Unlock()
	p.cond.Signal()
}

// discard drops a broken connection and frees its slot.
func (p *Pool) discard(c *Client) {
	if c != nil {
		_ = c.Close()
	}
	p.mu.Lock()
	p.open--
	p.cond.Broadcast()
	p.mu.Unlock()
}

// Exec runs one command sentence, reusing a pooled connection.
//
// If the pooled connection turns out to be dead (routers drop idle API
// sessions), the connection is replaced and the command retried once.
func (p *Pool) Exec(timeout time.Duration, words []string) ([]Sentence, error) {
	var lastErr error

	for attempt := 0; attempt < 2; attempt++ {
		c, err := p.acquire()
		if err != nil {
			return nil, err
		}
		if timeout > 0 {
			c.SetTimeout(timeout)
		}

		replies, err := c.RunCommand(words...)
		if err == nil {
			p.release(c)
			return replies, nil
		}

		lastErr = err
		p.discard(c)

		// A router-side rejection ("!trap" such as "already have user") is a
		// valid answer, not a broken connection: return it to the caller.
		if !isConnError(err) {
			return replies, err
		}
		// Connection-level failure: loop once to redial and retry.
	}

	return nil, lastErr
}

func (p *Pool) shutdown() {
	p.mu.Lock()
	p.closed = true
	idle := p.idle
	p.idle = nil
	p.open -= len(idle)
	p.mu.Unlock()

	for _, c := range idle {
		_ = c.Close()
	}
	p.cond.Broadcast()
}

// idleSince reports how long the pool has been unused; ok is false when the
// pool still holds a busy or idle connection and therefore must be kept.
func (p *Pool) idleSince() (time.Duration, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.open > 0 {
		return 0, false
	}
	return time.Since(p.lastUsed), true
}

func (p *Pool) stats() (open, idle int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.open, len(p.idle)
}
