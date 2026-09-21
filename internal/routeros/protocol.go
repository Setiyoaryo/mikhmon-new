package routeros

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type Client struct {
	conn    net.Conn
	mu      sync.Mutex
	timeout time.Duration
}

type Reply struct {
	Re   []map[string]string
	Done map[string]string
	Trap []map[string]string
}

func Dial(addr string, timeout time.Duration) (*Client, error) {
	if !strings.Contains(addr, ":") {
		addr = addr + ":8728"
	}
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(timeout))
	return &Client{conn: conn, timeout: timeout}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Login(user, pass string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	reply, err := c.sendAndRead([]string{"/login", "=name=" + user, "=password=" + pass})
	if err != nil {
		return err
	}
	// Post v6.43: just !done with no ret
	if reply.Done != nil {
		if _, ok := reply.Done["ret"]; !ok {
			return nil // success
		}
		// Pre v6.43: challenge-response
		challenge := reply.Done["ret"]
		if len(challenge) == 32 {
			resp := challengeResponse(pass, challenge)
			reply2, err := c.sendAndRead([]string{"/login", "=name=" + user, "=response=00" + resp})
			if err != nil {
				return err
			}
			if reply2.Done != nil {
				return nil
			}
			return fmt.Errorf("login failed (pre-v6.43)")
		}
	}
	if len(reply.Trap) > 0 {
		msg := ""
		if m, ok := reply.Trap[0]["message"]; ok {
			msg = m
		}
		return fmt.Errorf("login failed: %s", msg)
	}
	return nil
}

func challengeResponse(pass, challenge string) string {
	b, _ := hex.DecodeString(challenge)
	h := md5.New()
	h.Write([]byte{0})
	h.Write([]byte(pass))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Client) Run(command string, args ...string) (*Reply, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	words := append([]string{command}, args...)
	return c.sendAndRead(words)
}

func (c *Client) RunArgs(command string, args map[string]string) (*Reply, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	words := []string{command}
	for k, v := range args {
		if strings.HasPrefix(k, "?") || strings.HasPrefix(k, "~") {
			words = append(words, k+"="+v)
		} else {
			words = append(words, "="+k+"="+v)
		}
	}
	return c.sendAndRead(words)
}

func (c *Client) sendAndRead(words []string) (*Reply, error) {
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	for _, w := range words {
		if err := writeWord(c.conn, w); err != nil {
			return nil, err
		}
	}
	// send empty word to terminate sentence
	if err := writeWord(c.conn, ""); err != nil {
		return nil, err
	}
	return readReply(c.conn)
}

func writeWord(w io.Writer, word string) error {
	b := encodeLength(len(word))
	b = append(b, []byte(word)...)
	_, err := w.Write(b)
	return err
}

func encodeLength(l int) []byte {
	switch {
	case l < 0x80:
		return []byte{byte(l)}
	case l < 0x4000:
		return []byte{byte(l>>8) | 0x80, byte(l)}
	case l < 0x200000:
		return []byte{byte(l>>16) | 0xC0, byte(l >> 8), byte(l)}
	case l < 0x10000000:
		return []byte{byte(l>>24) | 0xE0, byte(l >> 16), byte(l >> 8), byte(l)}
	default:
		return []byte{0xF0, byte(l >> 24), byte(l >> 16), byte(l >> 8), byte(l)}
	}
}

func readReply(r io.Reader) (*Reply, error) {
	reply := &Reply{}
	var current map[string]string
	for {
		word, err := readWord(r)
		if err != nil {
			return reply, err
		}
		switch word {
		case "!re":
			current = make(map[string]string)
			reply.Re = append(reply.Re, current)
		case "!done":
			current = make(map[string]string)
			reply.Done = current
		case "!trap":
			current = make(map[string]string)
			reply.Trap = append(reply.Trap, current)
		case "!fatal":
			current = make(map[string]string)
			reply.Trap = append(reply.Trap, current)
		case "":
			// Empty word after !done means end of sentence
			if reply.Done != nil {
				return reply, nil
			}
		default:
			if current != nil && strings.HasPrefix(word, "=") {
				word = word[1:]
				idx := strings.Index(word, "=")
				if idx >= 0 {
					current[word[:idx]] = word[idx+1:]
				}
			}
		}
	}
}

func readWord(r io.Reader) (string, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	var length int
	first := b[0]
	switch {
	case first < 0x80:
		length = int(first)
	case first < 0xC0:
		b2 := make([]byte, 1)
		if _, err := io.ReadFull(r, b2); err != nil {
			return "", err
		}
		length = int(first&0x3F)<<8 | int(b2[0])
	case first < 0xE0:
		b2 := make([]byte, 2)
		if _, err := io.ReadFull(r, b2); err != nil {
			return "", err
		}
		length = int(first&0x1F)<<16 | int(b2[0])<<8 | int(b2[1])
	case first < 0xF0:
		b2 := make([]byte, 3)
		if _, err := io.ReadFull(r, b2); err != nil {
			return "", err
		}
		length = int(first&0x0F)<<24 | int(b2[0])<<16 | int(b2[1])<<8 | int(b2[2])
	default:
		b2 := make([]byte, 4)
		if _, err := io.ReadFull(r, b2); err != nil {
			return "", err
		}
		length = int(b2[0])<<24 | int(b2[1])<<16 | int(b2[2])<<8 | int(b2[3])
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
