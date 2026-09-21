package routeros

import (
	"fmt"
	"sync"
	"time"
)

type Pool struct {
	addr    string
	user    string
	pass    string
	timeout time.Duration
	mu      sync.Mutex
	clients []*Client
	maxSize int
}

func NewPool(addr, user, pass string, maxSize int, timeout time.Duration) *Pool {
	return &Pool{
		addr:    addr,
		user:    user,
		pass:    pass,
		timeout: timeout,
		maxSize: maxSize,
	}
}

func (p *Pool) Get() (*Client, error) {
	p.mu.Lock()
	if len(p.clients) > 0 {
		c := p.clients[len(p.clients)-1]
		p.clients = p.clients[:len(p.clients)-1]
		p.mu.Unlock()
		// test connection
		_, err := c.Run("/system/identity/print")
		if err == nil {
			return c, nil
		}
		c.Close()
	} else {
		p.mu.Unlock()
	}
	return p.connect()
}

func (p *Pool) Put(c *Client) {
	if c == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.clients) >= p.maxSize {
		c.Close()
		return
	}
	p.clients = append(p.clients, c)
}

func (p *Pool) connect() (*Client, error) {
	c, err := Dial(p.addr, p.timeout)
	if err != nil {
		return nil, fmt.Errorf("routeros dial %s: %w", p.addr, err)
	}
	if err := c.Login(p.user, p.pass); err != nil {
		c.Close()
		return nil, fmt.Errorf("routeros login: %w", err)
	}
	return c, nil
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.clients {
		c.Close()
	}
	p.clients = nil
}
