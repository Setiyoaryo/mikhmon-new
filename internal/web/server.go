package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mikhmon/go-mikhmon/internal/config"
	"github.com/mikhmon/go-mikhmon/internal/routeros"
)

type Server struct {
	cfg      *config.Config
	pools    map[string]*routeros.Pool
	poolsMu  sync.RWMutex
	sessions map[string]*Session
	sessMu   sync.RWMutex
	tmpl     *template.Template
}

type Session struct {
	ID        string
	User      string
	Router    string
	CreatedAt time.Time
	Data      map[string]interface{}
}

func NewServer(cfg *config.Config) *Server {
	s := &Server{
		cfg:      cfg,
		pools:    make(map[string]*routeros.Pool),
		sessions: make(map[string]*Session),
	}
	return s
}

func (s *Server) GetPool(sessionName string) (*routeros.Pool, error) {
	s.poolsMu.RLock()
	p, ok := s.pools[sessionName]
	s.poolsMu.RUnlock()
	if ok {
		return p, nil
	}

	rs := s.cfg.GetSession(sessionName)
	if rs == nil {
		return nil, fmt.Errorf("session %q not found", sessionName)
	}

	s.poolsMu.Lock()
	defer s.poolsMu.Unlock()
	// double check
	if p, ok := s.pools[sessionName]; ok {
		return p, nil
	}
	p = routeros.NewPool(rs.IP, rs.User, rs.Password, 10, 10*time.Second)
	s.pools[sessionName] = p
	return p, nil
}

func (s *Server) ClosePool(sessionName string) {
	s.poolsMu.Lock()
	defer s.poolsMu.Unlock()
	if p, ok := s.pools[sessionName]; ok {
		p.Close()
		delete(s.pools, sessionName)
	}
}

func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("mikhmon_session")
	if err != nil {
		return nil
	}
	s.sessMu.RLock()
	defer s.sessMu.RUnlock()
	return s.sessions[cookie.Value]
}

func (s *Server) createSession(w http.ResponseWriter, user string) *Session {
	id := generateSessionID()
	sess := &Session{
		ID:        id,
		User:      user,
		CreatedAt: time.Now(),
		Data:      make(map[string]interface{}),
	}
	s.sessMu.Lock()
	s.sessions[id] = sess
	s.sessMu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     "mikhmon_session",
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})
	return sess
}

func (s *Server) destroySession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("mikhmon_session")
	if err != nil {
		return
	}
	s.sessMu.Lock()
	delete(s.sessions, cookie.Value)
	s.sessMu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:   "mikhmon_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := s.getSession(r)
		if sess == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) jsonError(w http.ResponseWriter, status int, msg string) {
	s.jsonResponse(w, status, map[string]string{"error": msg})
}

// Dummy log suppression
func init() {
	_ = log.Println
}
