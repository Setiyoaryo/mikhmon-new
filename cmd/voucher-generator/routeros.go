package main

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
	"time"
)

// Sentence represents a single RouterOS reply sentence.
type Sentence []string

// Type returns the sentence type (!done, !trap, !re, !fatal).
func (s Sentence) Type() string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}

// Get returns the value of an attribute key (=key=value).
func (s Sentence) Get(key string) string {
	prefix := "=" + key + "="
	for _, w := range s {
		if strings.HasPrefix(w, prefix) {
			return strings.TrimPrefix(w, prefix)
		}
	}
	return ""
}

// Client is a connection to the MikroTik RouterOS API.
type Client struct {
	conn    net.Conn
	reader  *bufio.Reader
	writer  *bufio.Writer
	timeout time.Duration
}

// Dial connects to the RouterOS API port with an optional timeout.
func Dial(addr string, useSSL bool, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	var conn net.Conn
	var err error

	if useSSL {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		dialer := &net.Dialer{Timeout: timeout}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = net.DialTimeout("tcp", addr, timeout)
	}

	if err != nil {
		return nil, err
	}

	return &Client{
		conn:    conn,
		reader:  bufio.NewReaderSize(conn, 32768),
		writer:  bufio.NewWriterSize(conn, 32768),
		timeout: timeout,
	}, nil
}

// Close closes the underlying network connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// encodeLength encodes length according to RouterOS API protocol.
func encodeLength(l int) []byte {
	if l < 0x80 {
		return []byte{byte(l)}
	} else if l < 0x4000 {
		return []byte{byte((l >> 8) | 0x80), byte(l & 0xFF)}
	} else if l < 0x200000 {
		return []byte{byte((l >> 16) | 0xC0), byte((l >> 8) & 0xFF), byte(l & 0xFF)}
	} else if l < 0x10000000 {
		return []byte{byte((l >> 24) | 0xE0), byte((l >> 16) & 0xFF), byte((l >> 8) & 0xFF), byte(l & 0xFF)}
	} else {
		return []byte{0xF0, byte((l >> 24) & 0xFF), byte((l >> 16) & 0xFF), byte((l >> 8) & 0xFF), byte(l & 0xFF)}
	}
}

// decodeLength reads length from the stream.
func decodeLength(r io.Reader) (int, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	b1 := b[0]
	if b1&0x80 == 0 {
		return int(b1), nil
	} else if (b1 & 0xC0) == 0x80 {
		var b2 [1]byte
		if _, err := io.ReadFull(r, b2[:]); err != nil {
			return 0, err
		}
		return int(b1&0x3F)<<8 | int(b2[0]), nil
	} else if (b1 & 0xE0) == 0xC0 {
		var rest [2]byte
		if _, err := io.ReadFull(r, rest[:]); err != nil {
			return 0, err
		}
		return int(b1&0x1F)<<16 | int(rest[0])<<8 | int(rest[1]), nil
	} else if (b1 & 0xF0) == 0xE0 {
		var rest [3]byte
		if _, err := io.ReadFull(r, rest[:]); err != nil {
			return 0, err
		}
		return int(b1&0x0F)<<24 | int(rest[0])<<16 | int(rest[1])<<8 | int(rest[2]), nil
	} else if b1 == 0xF0 {
		var rest [4]byte
		if _, err := io.ReadFull(r, rest[:]); err != nil {
			return 0, err
		}
		return int(rest[0])<<24 | int(rest[1])<<16 | int(rest[2])<<8 | int(rest[3]), nil
	}
	return 0, fmt.Errorf("invalid length prefix byte: 0x%x", b1)
}

// writeSentence writes a sentence to the socket.
func (c *Client) writeSentence(words ...string) error {
	if c.timeout > 0 {
		_ = c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	}
	for _, word := range words {
		if _, err := c.writer.Write(encodeLength(len(word))); err != nil {
			return err
		}
		if _, err := c.writer.WriteString(word); err != nil {
			return err
		}
	}
	if err := c.writer.WriteByte(0x00); err != nil {
		return err
	}
	return c.writer.Flush()
}

// readSentence reads a single sentence from the socket.
func (c *Client) readSentence() (Sentence, error) {
	if c.timeout > 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	}
	var words []string
	for {
		l, err := decodeLength(c.reader)
		if err != nil {
			return nil, err
		}
		if l == 0 {
			break // end of sentence
		}
		buf := make([]byte, l)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return nil, err
		}
		words = append(words, string(buf))
	}
	return words, nil
}

// Login performs authentication supporting both post-6.43 and pre-6.43 RouterOS versions.
func (c *Client) Login(username, password string) error {
	// Try post-v6.43 login first
	err := c.writeSentence("/login", "=name="+username, "=password="+password)
	if err != nil {
		return fmt.Errorf("failed to send login command: %w", err)
	}

	reply, err := c.readSentence()
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	if reply.Type() == "!trap" {
		msg := reply.Get("message")
		if msg == "" {
			msg = "authentication failed"
		}
		return errors.New(msg)
	}

	if reply.Type() == "!done" {
		challenge := reply.Get("ret")
		if challenge == "" {
			// Modern login succeeded
			return nil
		}

		// Legacy pre-v6.43 challenge-response login
		chalBytes, err := hex.DecodeString(challenge)
		if err != nil {
			return fmt.Errorf("invalid challenge hex: %w", err)
		}

		hasher := md5.New()
		hasher.Write([]byte{0})
		hasher.Write([]byte(password))
		hasher.Write(chalBytes)
		responseHex := hex.EncodeToString(hasher.Sum(nil))

		err = c.writeSentence("/login", "=name="+username, "=response=00"+responseHex)
		if err != nil {
			return fmt.Errorf("failed to send legacy login response: %w", err)
		}

		reply2, err := c.readSentence()
		if err != nil {
			return fmt.Errorf("failed to read legacy login response: %w", err)
		}

		if reply2.Type() == "!done" {
			return nil
		}

		if reply2.Type() == "!trap" {
			msg := reply2.Get("message")
			if msg == "" {
				msg = "authentication failed"
			}
			return errors.New(msg)
		}

		return fmt.Errorf("unexpected legacy login reply: %v", reply2)
	}

	return fmt.Errorf("unexpected login reply: %v", reply)
}

// RunCommand sends a command and returns all reply sentences until !done or !fatal.
func (c *Client) RunCommand(words ...string) ([]Sentence, error) {
	if err := c.writeSentence(words...); err != nil {
		return nil, err
	}

	var replies []Sentence
	var trapErr error

	for {
		sentence, err := c.readSentence()
		if err != nil {
			return nil, err
		}
		replies = append(replies, sentence)

		switch sentence.Type() {
		case "!trap":
			msg := sentence.Get("message")
			if msg == "" {
				msg = "command failed"
			}
			trapErr = errors.New(msg)
		case "!fatal":
			msg := sentence.Get("message")
			if msg == "" {
				msg = "fatal routeros error"
			}
			return replies, errors.New(msg)
		case "!done":
			if trapErr != nil {
				return replies, trapErr
			}
			return replies, nil
		}
	}
}

// DialAndLogin connects and authenticates with the router.
func DialAndLogin(addr, username, password string, useSSL bool, timeout time.Duration) (*Client, error) {
	client, err := Dial(addr, useSSL, timeout)
	if err != nil {
		return nil, err
	}

	if err := client.Login(username, password); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}
