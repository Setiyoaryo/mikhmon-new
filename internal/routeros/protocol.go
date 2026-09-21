// Package routeros implements the MikroTik RouterOS API wire protocol
// (both the post-6.43 plain login and the pre-6.43 challenge/response login)
// plus a reusable connection pool that the PHP frontend talks to over HTTP.
package routeros

import (
	"bufio"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// Sentence is a single RouterOS reply sentence, e.g.
//
//	["!re", "=name=foo", "=.id=*1"]
//
// The first word is the sentence type ("!done", "!re", "!trap", "!fatal").
type Sentence []string

// Type returns the sentence type word, or "" when the sentence is empty.
func (s Sentence) Type() string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}

// Get returns the value of an attribute word ("=key=value").
func (s Sentence) Get(key string) string {
	prefix := "=" + key + "="
	for _, w := range s {
		if strings.HasPrefix(w, prefix) {
			return strings.TrimPrefix(w, prefix)
		}
	}
	return ""
}

// IsTrap reports whether the sentence is an error reply.
func (s Sentence) IsTrap() bool {
	t := s.Type()
	return t == "!trap" || t == "!fatal"
}

// Client is a single authenticated connection to the RouterOS API.
//
// A Client is NOT safe for concurrent use: the pool hands a Client to exactly
// one caller at a time.
type Client struct {
	conn    net.Conn
	reader  *bufio.Reader
	writer  *bufio.Writer
	timeout time.Duration
	mu      sync.Mutex // guards timeout only
}

// Dial connects to the RouterOS API endpoint.
func Dial(addr string, useSSL bool, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	var conn net.Conn
	var err error

	if useSSL {
		dialer := &net.Dialer{Timeout: timeout}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true, // routers use self-signed certificates
		})
	} else {
		conn, err = net.DialTimeout("tcp", addr, timeout)
	}
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:    conn,
		reader:  bufio.NewReaderSize(conn, 64*1024),
		writer:  bufio.NewWriterSize(conn, 64*1024),
		timeout: timeout,
	}, nil
}

// SetTimeout changes the per-operation read/write deadline.
func (c *Client) SetTimeout(d time.Duration) {
	if d <= 0 {
		return
	}
	c.mu.Lock()
	c.timeout = d
	c.mu.Unlock()
}

func (c *Client) getTimeout() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.timeout
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// encodeLength encodes a word length using the RouterOS variable-length scheme.
func encodeLength(l int) []byte {
	switch {
	case l < 0x80:
		return []byte{byte(l)}
	case l < 0x4000:
		return []byte{byte((l >> 8) | 0x80), byte(l & 0xFF)}
	case l < 0x200000:
		return []byte{byte((l >> 16) | 0xC0), byte((l >> 8) & 0xFF), byte(l & 0xFF)}
	case l < 0x10000000:
		return []byte{byte((l >> 24) | 0xE0), byte((l >> 16) & 0xFF), byte((l >> 8) & 0xFF), byte(l & 0xFF)}
	default:
		return []byte{0xF0, byte((l >> 24) & 0xFF), byte((l >> 16) & 0xFF), byte((l >> 8) & 0xFF), byte(l & 0xFF)}
	}
}

// decodeLength reads a variable-length word length from the stream.
func decodeLength(r io.Reader) (int, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	b1 := b[0]

	switch {
	case b1&0x80 == 0:
		return int(b1), nil
	case b1&0xC0 == 0x80:
		var rest [1]byte
		if _, err := io.ReadFull(r, rest[:]); err != nil {
			return 0, err
		}
		return int(b1&0x3F)<<8 | int(rest[0]), nil
	case b1&0xE0 == 0xC0:
		var rest [2]byte
		if _, err := io.ReadFull(r, rest[:]); err != nil {
			return 0, err
		}
		return int(b1&0x1F)<<16 | int(rest[0])<<8 | int(rest[1]), nil
	case b1&0xF0 == 0xE0:
		var rest [3]byte
		if _, err := io.ReadFull(r, rest[:]); err != nil {
			return 0, err
		}
		return int(b1&0x0F)<<24 | int(rest[0])<<16 | int(rest[1])<<8 | int(rest[2]), nil
	case b1 == 0xF0:
		var rest [4]byte
		if _, err := io.ReadFull(r, rest[:]); err != nil {
			return 0, err
		}
		return int(rest[0])<<24 | int(rest[1])<<16 | int(rest[2])<<8 | int(rest[3]), nil
	}
	return 0, fmt.Errorf("routeros: invalid length prefix byte 0x%x", b1)
}

// writeSentence sends one complete sentence (terminated by a zero byte).
func (c *Client) writeSentence(words ...string) error {
	if d := c.getTimeout(); d > 0 {
		_ = c.conn.SetWriteDeadline(time.Now().Add(d))
	}
	for _, w := range words {
		if _, err := c.writer.Write(encodeLength(len(w))); err != nil {
			return err
		}
		if _, err := c.writer.WriteString(w); err != nil {
			return err
		}
	}
	if err := c.writer.WriteByte(0x00); err != nil {
		return err
	}
	return c.writer.Flush()
}

// readSentence reads one complete sentence.
func (c *Client) readSentence() (Sentence, error) {
	if d := c.getTimeout(); d > 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(d))
	}
	var words []string
	for {
		l, err := decodeLength(c.reader)
		if err != nil {
			return nil, err
		}
		if l == 0 {
			return words, nil
		}
		buf := make([]byte, l)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return nil, err
		}
		words = append(words, string(buf))
	}
}

// Login authenticates, supporting both post-6.43 and pre-6.43 RouterOS.
func (c *Client) Login(username, password string) error {
	err := c.writeSentence("/login", "=name="+username, "=password="+password)
	if err != nil {
		return fmt.Errorf("send login: %w", err)
	}

	reply, err := c.readSentence()
	if err != nil {
		return fmt.Errorf("read login reply: %w", err)
	}

	switch reply.Type() {
	case "!trap", "!fatal":
		return errors.New(loginMessage(reply))
	case "!done":
		challenge := reply.Get("ret")
		if challenge == "" {
			return nil // modern login
		}
		return c.legacyLogin(username, password, challenge)
	default:
		return fmt.Errorf("unexpected login reply: %v", []string(reply))
	}
}

// legacyLogin performs the pre-6.43 md5 challenge/response handshake.
func (c *Client) legacyLogin(username, password, challenge string) error {
	chalBytes, err := hex.DecodeString(challenge)
	if err != nil {
		return fmt.Errorf("invalid challenge: %w", err)
	}

	h := md5.New()
	h.Write([]byte{0})
	h.Write([]byte(password))
	h.Write(chalBytes)
	response := "00" + hex.EncodeToString(h.Sum(nil))

	if err := c.writeSentence("/login", "=name="+username, "=response="+response); err != nil {
		return fmt.Errorf("send legacy login: %w", err)
	}

	reply, err := c.readSentence()
	if err != nil {
		return fmt.Errorf("read legacy login reply: %w", err)
	}

	switch reply.Type() {
	case "!done":
		return nil
	case "!trap", "!fatal":
		return errors.New(loginMessage(reply))
	default:
		return fmt.Errorf("unexpected legacy login reply: %v", []string(reply))
	}
}

func loginMessage(s Sentence) string {
	if m := s.Get("message"); m != "" {
		return m
	}
	return "authentication failed"
}

// RunCommand sends one command sentence and collects every reply sentence up
// to (and including) the terminating "!done".
//
// The raw sentences are returned even when the command failed, so callers can
// inspect "!trap" details. A non-nil error means the command did not complete.
func (c *Client) RunCommand(words ...string) ([]Sentence, error) {
	if len(words) == 0 {
		return nil, errors.New("routeros: empty command")
	}
	if err := c.writeSentence(words...); err != nil {
		return nil, err
	}

	var (
		replies []Sentence
		trapErr error
	)
	for {
		s, err := c.readSentence()
		if err != nil {
			return replies, err
		}
		replies = append(replies, s)

		switch s.Type() {
		case "!trap":
			if trapErr == nil {
				trapErr = errors.New(loginMessage(s))
			}
		case "!fatal":
			return replies, errors.New(loginMessage(s))
		case "!done":
			return replies, trapErr
		}
	}
}

// DialAndLogin connects and authenticates in one step.
func DialAndLogin(addr, username, password string, useSSL bool, timeout time.Duration) (*Client, error) {
	c, err := Dial(addr, useSSL, timeout)
	if err != nil {
		return nil, err
	}
	if err := c.Login(username, password); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}
