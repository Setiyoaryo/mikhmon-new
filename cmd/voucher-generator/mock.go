package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// MockRouterOSServer implements a mock RouterOS API server for testing.
type MockRouterOSServer struct {
	listener net.Listener
	Addr     string
	mu       sync.Mutex
	Users    map[string]Voucher
	UserCnt  int64
	legacy   bool
	dupOnce  sync.Map
}

// NewMockServer creates and starts a new mock RouterOS API server.
func NewMockServer(addr string, legacy bool) (*MockRouterOSServer, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	s := &MockRouterOSServer{
		listener: ln,
		Addr:     ln.Addr().String(),
		Users:    make(map[string]Voucher),
		legacy:   legacy,
	}

	go s.serve()
	return s, nil
}

// Close stops the mock server.
func (s *MockRouterOSServer) Close() error {
	return s.listener.Close()
}

func (s *MockRouterOSServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *MockRouterOSServer) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReaderSize(conn, 32768)
	authenticated := false

	for {
		var words []string
		for {
			l, err := decodeLength(reader)
			if err != nil {
				return
			}
			if l == 0 {
				break
			}
			b := make([]byte, l)
			if _, err := io.ReadFull(reader, b); err != nil {
				return
			}
			words = append(words, string(b))
		}

		if len(words) == 0 {
			continue
		}

		cmd := words[0]

		if !authenticated {
			if cmd != "/login" {
				_ = writeMockSentence(conn, "!trap", "=message=not logged in")
				_ = writeMockSentence(conn, "!done")
				return
			}

			if s.legacy {
				resp := getAttr(words, "response")
				if resp == "" {
					challenge := "0123456789abcdef0123456789abcdef"
					_ = writeMockSentence(conn, "!done", "=ret="+challenge)
					continue
				}

				user := getAttr(words, "name")
				hasher := md5.New()
				hasher.Write([]byte{0})
				hasher.Write([]byte("secret"))
				chalBytes, _ := hex.DecodeString("0123456789abcdef0123456789abcdef")
				hasher.Write(chalBytes)
				expectedResp := "00" + hex.EncodeToString(hasher.Sum(nil))

				if user == "admin" && resp == expectedResp {
					authenticated = true
					_ = writeMockSentence(conn, "!done")
				} else {
					_ = writeMockSentence(conn, "!trap", "=message=invalid username or password")
					_ = writeMockSentence(conn, "!done")
					return
				}
				continue
			}

			// Modern login
			user := getAttr(words, "name")
			pass := getAttr(words, "password")
			if (user == "admin" && pass == "secret") || pass == "admin" || pass == "" {
				authenticated = true
				_ = writeMockSentence(conn, "!done")
			} else {
				_ = writeMockSentence(conn, "!trap", "=message=invalid username or password")
				_ = writeMockSentence(conn, "!done")
				return
			}
			continue
		}

		// Authenticated commands
		switch cmd {
		case "/ip/hotspot/user/add":
			name := getAttr(words, "name")
			pass := getAttr(words, "password")
			server := getAttr(words, "server")
			profile := getAttr(words, "profile")
			timelimit := getAttr(words, "limit-uptime")
			datalimitStr := getAttr(words, "limit-bytes-total")
			datalimit, _ := strconv.ParseInt(datalimitStr, 10, 64)
			comment := getAttr(words, "comment")

			// Simulate duplicate detection on trigger string
			if strings.Contains(name, "force_dup") {
				if _, loaded := s.dupOnce.LoadOrStore(name, true); !loaded {
					_ = writeMockSentence(conn, "!trap", "=message=already have user with this name")
					_ = writeMockSentence(conn, "!done")
					continue
				}
			}

			s.mu.Lock()
			if _, exists := s.Users[name]; exists {
				s.mu.Unlock()
				_ = writeMockSentence(conn, "!trap", "=message=already have user with this name")
				_ = writeMockSentence(conn, "!done")
				continue
			}
			s.Users[name] = Voucher{
				Name:      name,
				Password:  pass,
				Server:    server,
				Profile:   profile,
				TimeLimit: timelimit,
				DataLimit: datalimit,
				Comment:   comment,
			}
			s.mu.Unlock()

			atomic.AddInt64(&s.UserCnt, 1)
			_ = writeMockSentence(conn, "!done")

		default:
			_ = writeMockSentence(conn, "!done")
		}
	}
}

func getAttr(words []string, key string) string {
	prefix := "=" + key + "="
	for _, w := range words {
		if strings.HasPrefix(w, prefix) {
			return strings.TrimPrefix(w, prefix)
		}
	}
	return ""
}

func writeMockSentence(w io.Writer, words ...string) error {
	var buf bytes.Buffer
	for _, word := range words {
		buf.Write(encodeLength(len(word)))
		buf.WriteString(word)
	}
	buf.WriteByte(0x00)
	_, err := w.Write(buf.Bytes())
	return err
}

// RunMockServerCLI runs the mock server until interrupted.
func RunMockServerCLI(addr string) {
	s, err := NewMockServer(addr, false)
	if err != nil {
		log.Fatalf("failed to start mock server: %v", err)
	}
	log.Printf("Mock RouterOS API server listening on %s", s.Addr)
	select {}
}
