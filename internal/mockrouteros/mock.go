// Package mockrouteros implements a fake RouterOS API server.
//
// It exists so the Go service and the PHP frontend can be exercised end to end
// without a real MikroTik device. Run it with cmd/mockrouteros.
package mockrouteros

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// User is one hotspot user stored by the mock.
type User struct {
	ID      string
	Name    string
	Pass    string
	Server  string
	Profile string
	Uptime  string
	Comment string
}

// Options configures the mock server.
type Options struct {
	// Username/Password are the accepted credentials.
	Username string
	Password string
	// Legacy forces the pre-6.43 challenge/response login flow.
	Legacy bool
	// Quiet disables per-command logging.
	Quiet bool
}

// Server is a fake RouterOS API endpoint.
type Server struct {
	ln   net.Listener
	opts Options

	mu       sync.Mutex
	users    map[string]User
	seq      int
	commands map[string]int
}

// Start listens on addr (":0" picks a free port) and serves in the background.
func Start(addr string, opts Options) (*Server, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	if opts.Username == "" {
		opts.Username = "admin"
	}
	s := &Server{
		ln:       ln,
		opts:     opts,
		users:    make(map[string]User),
		commands: make(map[string]int),
	}
	go s.serve()
	return s, nil
}

// Addr returns the bound address.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Port returns the bound port.
func (s *Server) Port() int {
	if a, ok := s.ln.Addr().(*net.TCPAddr); ok {
		return a.Port
	}
	return 0
}

// Close stops the listener.
func (s *Server) Close() error { return s.ln.Close() }

// Users returns a sorted snapshot of the stored users.
func (s *Server) Users() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// UserCount returns how many users were created.
func (s *Server) UserCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.users)
}

// CommandCount returns how many times a command path was invoked.
func (s *Server) CommandCount(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commands[path]
}

func (s *Server) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReaderSize(conn, 64*1024)
	authed := false

	for {
		words, err := readSentence(r)
		if err != nil {
			return
		}
		if len(words) == 0 {
			continue
		}

		if !authed {
			if words[0] != "/login" {
				_ = write(conn, "!trap", "=message=not logged in")
				_ = write(conn, "!done")
				return
			}
			done, ok := s.handleLogin(conn, words)
			if !ok {
				return
			}
			if done {
				authed = true
			}
			continue
		}

		s.handleCommand(conn, words)
	}
}

// handleLogin returns (loginFinished, connectionUsable).
func (s *Server) handleLogin(conn net.Conn, words []string) (bool, bool) {
	const challenge = "0123456789abcdef0123456789abcdef"

	if s.opts.Legacy && attr(words, "response") == "" {
		_ = write(conn, "!done", "=ret="+challenge)
		return false, true
	}

	if s.opts.Legacy {
		chalBytes, _ := hex.DecodeString(challenge)
		h := md5.New()
		h.Write([]byte{0})
		h.Write([]byte(s.opts.Password))
		h.Write(chalBytes)
		expected := "00" + hex.EncodeToString(h.Sum(nil))

		if attr(words, "name") != s.opts.Username || attr(words, "response") != expected {
			_ = write(conn, "!trap", "=message=invalid username or password")
			_ = write(conn, "!done")
			return false, false
		}
		_ = write(conn, "!done")
		return true, true
	}

	if attr(words, "name") != s.opts.Username || attr(words, "password") != s.opts.Password {
		_ = write(conn, "!trap", "=message=invalid username or password")
		_ = write(conn, "!done")
		return false, false
	}
	_ = write(conn, "!done")
	return true, true
}

func (s *Server) handleCommand(conn net.Conn, words []string) {
	path := words[0]

	s.mu.Lock()
	s.commands[path]++
	s.mu.Unlock()

	if !s.opts.Quiet {
		log.Printf("mockrouteros: %s", path)
	}

	switch path {
	case "/ip/hotspot/user/add":
		s.cmdUserAdd(conn, words)
	case "/ip/hotspot/user/print":
		s.cmdUserPrint(conn, words)
	case "/ip/hotspot/user/remove":
		s.cmdUserRemove(conn, words)
	case "/ip/hotspot/user/profile/print":
		_ = write(conn, "!re", "=.id=*1", "=name=default", "=shared-users=1", "=on-login=:::0,0,0,0,,")
		_ = write(conn, "!done")
	case "/ip/hotspot/print":
		_ = write(conn, "!re", "=.id=*1", "=name=all")
		_ = write(conn, "!done")
	case "/system/identity/print":
		_ = write(conn, "!re", "=name=mock-router")
		_ = write(conn, "!done")
	case "/system/resource/print":
		_ = write(conn, "!re",
			"=uptime=1w2d3h",
			"=version=7.14.3",
			"=cpu-load=3",
			"=free-memory=123456789",
			"=total-memory=268435456",
			"=free-hdd-space=12345678",
			"=total-hdd-space=67108864",
			"=board-name=RB750Gr3",
		)
		_ = write(conn, "!done")
	case "/system/clock/print":
		_ = write(conn, "!re", "=date=2024-01-01", "=time=00:00:00", "=time-zone-name=Asia/Jakarta")
		_ = write(conn, "!done")
	case "/system/reboot", "/system/shutdown":
		// Real routers drop the connection instead of replying.
		_ = conn.Close()
	default:
		_ = write(conn, "!done")
	}
}

func (s *Server) cmdUserAdd(conn net.Conn, words []string) {
	name := attr(words, "name")

	s.mu.Lock()
	if _, exists := s.users[name]; exists {
		s.mu.Unlock()
		_ = write(conn, "!trap", "=message=failure: already have user with this name for this server")
		_ = write(conn, "!done")
		return
	}
	s.seq++
	s.users[name] = User{
		ID:      "*" + strconv.Itoa(s.seq),
		Name:    name,
		Pass:    attr(words, "password"),
		Server:  attr(words, "server"),
		Profile: attr(words, "profile"),
		Uptime:  "0s",
		Comment: attr(words, "comment"),
	}
	s.mu.Unlock()

	_ = write(conn, "!done")
}

func (s *Server) cmdUserPrint(conn net.Conn, words []string) {
	s.mu.Lock()
	users := make([]User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	total := len(users)
	s.mu.Unlock()

	if hasQuery(words, "count-only") {
		_ = write(conn, "!done", "=ret="+strconv.Itoa(total))
		return
	}

	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })

	if name := queryValue(words, "name"); name != "" {
		filtered := make([]User, 0, len(users))
		for _, u := range users {
			if u.Name == name {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}
	if comment := queryValue(words, "comment"); comment != "" {
		filtered := make([]User, 0, len(users))
		for _, u := range users {
			if u.Comment == comment {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}

	for _, u := range users {
		_ = write(conn, "!re",
			"=.id="+u.ID,
			"=name="+u.Name,
			"=password="+u.Pass,
			"=profile="+u.Profile,
			"=uptime="+u.Uptime,
			"=bytes-in=0",
			"=bytes-out=0",
			"=comment="+u.Comment,
		)
	}
	_ = write(conn, "!done")
}

func (s *Server) cmdUserRemove(conn net.Conn, words []string) {
	id := attr(words, ".id")
	s.mu.Lock()
	for name, u := range s.users {
		if u.ID == id {
			delete(s.users, name)
			break
		}
	}
	s.mu.Unlock()
	_ = write(conn, "!done")
}

// ------------------------------------------------------------- wire codec

// readSentence reads one complete sentence of words.
func readSentence(r *bufio.Reader) ([]string, error) {
	var words []string
	for {
		l, err := decodeLength(r)
		if err != nil {
			return nil, err
		}
		if l == 0 {
			return words, nil
		}
		buf := make([]byte, l)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		words = append(words, string(buf))
	}
}

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
	return 0, fmt.Errorf("invalid length prefix 0x%x", b1)
}

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

func write(w io.Writer, words ...string) error {
	var sb strings.Builder
	for _, word := range words {
		sb.Write(encodeLength(len(word)))
		sb.WriteString(word)
	}
	sb.WriteByte(0x00)
	_, err := io.WriteString(w, sb.String())
	return err
}

// attr reads an "=key=value" attribute word.
func attr(words []string, key string) string {
	prefix := "=" + key + "="
	for _, w := range words {
		if strings.HasPrefix(w, prefix) {
			return strings.TrimPrefix(w, prefix)
		}
	}
	return ""
}

// hasQuery reports whether the command carries a count-only request.
func hasQuery(words []string, key string) bool {
	for _, w := range words {
		if w == "="+key+"=" || w == "="+key+"=yes" {
			return true
		}
	}
	return false
}

// queryValue reads a "?key=value" query word.
func queryValue(words []string, key string) string {
	prefix := "?" + key + "="
	for _, w := range words {
		if strings.HasPrefix(w, prefix) {
			return strings.TrimPrefix(w, prefix)
		}
	}
	return ""
}
