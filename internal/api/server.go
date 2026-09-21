// Package api exposes the RouterOS connection pool to the Mikhmon PHP
// frontend over HTTP. PHP keeps rendering 100% of the original UI; only the
// RouterOS traffic moves into this process, which is what makes connection
// reuse and concurrent voucher generation possible.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Setiyoaryo/mikhmon-new/internal/generator"
	"github.com/Setiyoaryo/mikhmon-new/internal/routeros"
)

// Options configures the HTTP server.
type Options struct {
	Addr             string
	Token            string
	MaxConnPerRouter int
	IdleTimeout      time.Duration
	SessionTTL       time.Duration
	ReadTimeout      time.Duration
}

// Server wraps the routeros.Manager with HTTP handlers.
type Server struct {
	opts Options
	mgr  *routeros.Manager
}

// New builds a Server.
func New(opts Options) *Server {
	if opts.MaxConnPerRouter < 1 {
		opts.MaxConnPerRouter = 32
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = 60 * time.Second
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 10 * time.Minute
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = 15 * time.Second
	}

	return &Server{
		opts: opts,
		mgr: routeros.NewManager(routeros.ManagerOptions{
			MaxConnPerRouter: opts.MaxConnPerRouter,
			IdleTimeout:      opts.IdleTimeout,
			SessionTTL:       opts.SessionTTL,
		}),
	}
}

// Close releases every pooled connection.
func (s *Server) Close() { s.mgr.Close() }

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/connect", s.handleConnect)
	mux.HandleFunc("/v1/disconnect", s.handleDisconnect)
	mux.HandleFunc("/v1/exec", s.handleExec)
	mux.HandleFunc("/v1/generate", s.handleGenerate)
	mux.HandleFunc("/v1/bulk/user-add", s.handleBulkUserAdd)
	return s.withRecovery(s.withAuth(mux))
}

func (s *Server) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic serving %s: %v", r.URL.Path, rec)
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"ok":    false,
					"error": "internal error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.opts.Token == "" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		got := r.Header.Get("X-Mikhmon-Token")
		if got == "" {
			got = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		if got != s.opts.Token {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"status": "up",
		"stats":  s.mgr.Stats(),
	})
}

// ---------------------------------------------------------------- connect

type connectRequest struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Pass      string `json:"pass"`
	SSL       bool   `json:"ssl"`
	TimeoutMS int64  `json:"timeout_ms"`
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req connectRequest
	if !decodeBody(w, r, &req) {
		return
	}

	target, err := buildTarget(req)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	sess, err := s.mgr.Connect(target)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "session": sess.ID})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	// Sessions are cheap and pooled TCP connections are deliberately kept
	// alive: PHP calls connect()/disconnect() on every request, and keeping the
	// socket warm here is the entire point of this service.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func buildTarget(req connectRequest) (routeros.Target, error) {
	host := strings.TrimSpace(req.Host)
	if host == "" {
		return routeros.Target{}, errors.New("missing host")
	}

	port := req.Port
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		if parsed, cerr := strconv.Atoi(p); cerr == nil && parsed > 0 {
			port = parsed
		}
	}
	if port <= 0 {
		if req.SSL {
			port = 8729
		} else {
			port = 8728
		}
	}

	timeout := 10 * time.Second
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}
	// The original PHP class used a 3s socket timeout. Keep a sane floor so a
	// black-holed router cannot pin an FPM worker forever.
	if timeout < time.Second {
		timeout = time.Second
	}

	return routeros.Target{
		Addr:     net.JoinHostPort(host, strconv.Itoa(port)),
		Username: req.User,
		Password: req.Pass,
		UseSSL:   req.SSL,
		Timeout:  timeout,
	}, nil
}

// ------------------------------------------------------------------- exec

type execRequest struct {
	Session   string     `json:"session"`
	Sentences [][]string `json:"sentences"`
	TimeoutMS int64      `json:"timeout_ms"`
}

type execResult struct {
	Sentences [][]string `json:"sentences"`
	Error     string     `json:"error,omitempty"`
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	var req execRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Sentences) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "results": []execResult{}})
		return
	}

	// Reject an unknown/stale session up front so the single-sentence and batch
	// paths behave identically.
	if _, err := s.requireSession(req.Session); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	timeout := s.execTimeout(req.TimeoutMS)

	// One sentence is the overwhelmingly common case (every $API->comm() call),
	// so handle it inline and skip goroutine overhead on the hot path.
	if len(req.Sentences) == 1 {
		replies, err := s.mgr.Exec(req.Session, timeout, req.Sentences[0])
		writeExecResponse(w, []execResult{{
			Sentences: toRaw(replies),
			Error:     tolerantError(req.Sentences[0], err),
		}})
		return
	}

	errs, err := s.mgr.ExecBatch(req.Session, len(req.Sentences), timeout, req.Sentences)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	results := make([]execResult, len(req.Sentences))
	for i := range req.Sentences {
		results[i] = execResult{Error: tolerantError(req.Sentences[i], errs[i])}
	}
	writeExecResponse(w, results)
}

func (s *Server) execTimeout(ms int64) time.Duration {
	if ms <= 0 {
		return s.opts.ReadTimeout
	}
	return time.Duration(ms) * time.Millisecond
}

func writeExecResponse(w http.ResponseWriter, results []execResult) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "results": results})
}

func toRaw(sentences []routeros.Sentence) [][]string {
	out := make([][]string, len(sentences))
	for i, s := range sentences {
		out[i] = []string(s)
	}
	return out
}

// tolerantError special-cases the two commands that intentionally kill the API
// connection: /system/reboot and /system/shutdown. PHP wrote them and then
// called read(), so a dropped socket must not surface as a failure.
func tolerantError(words []string, err error) string {
	if err == nil {
		return ""
	}
	if len(words) > 0 {
		if cmd := words[0]; cmd == "/system/reboot" || cmd == "/system/shutdown" {
			return ""
		}
	}
	return err.Error()
}

// -------------------------------------------------------------- generate

type generateRequest struct {
	Session     string   `json:"session"`
	Qty         int      `json:"qty"`
	Server      string   `json:"server"`
	Mode        string   `json:"mode"`
	Prefix      string   `json:"prefix"`
	CharType    string   `json:"char"`
	Profile     string   `json:"profile"`
	TimeLimit   string   `json:"timelimit"`
	DataLimit   int64    `json:"datalimit"`
	Comment     string   `json:"comment"`
	UserLength  int      `json:"userl"`
	Concurrency int      `json:"concurrency"`
	TimeoutMS   int64    `json:"timeout_ms"`
	DryRun      bool     `json:"dry_run"`
	Existing    []string `json:"existing"`
}

// handleGenerate creates qty hotspot users concurrently.
//
// This is the fast path behind the "Generate" button: one HTTP request instead
// of qty sequential RouterOS round-trips.
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req generateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	s.runGenerate(w, req)
}

func (s *Server) runGenerate(w http.ResponseWriter, req generateRequest) {
	start := time.Now()

	gen := generator.New()
	for _, e := range req.Existing {
		gen.MarkUsed(e)
	}

	vouchers := gen.Batch(generator.BatchOptions{
		Qty:        clampQty(req.Qty),
		Server:     req.Server,
		Mode:       generator.ParseMode(req.Mode),
		Prefix:     req.Prefix,
		CharType:   generator.ValidateCharType(req.CharType),
		Profile:    req.Profile,
		TimeLimit:  req.TimeLimit,
		DataLimit:  req.DataLimit,
		Comment:    req.Comment,
		UserLength: generator.ClampUserLength(req.UserLength),
	})

	if req.DryRun || req.Session == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"total":       len(vouchers),
			"added":       0,
			"failed":      0,
			"dry_run":     true,
			"duration_ms": time.Since(start).Milliseconds(),
			"first_user":  firstName(vouchers),
			"last_user":   lastName(vouchers),
			"vouchers":    vouchers,
		})
		return
	}

	report := s.addVouchers(req.Session, req.Concurrency, s.execTimeout(req.TimeoutMS), vouchers)
	report["ok"] = true
	report["duration_ms"] = time.Since(start).Milliseconds()
	report["vouchers"] = vouchers
	writeJSON(w, http.StatusOK, report)
}

// -------------------------------------------------------------- bulk add

type bulkUserAddRequest struct {
	Session     string              `json:"session"`
	Users       []generator.Voucher `json:"users"`
	Concurrency int                 `json:"concurrency"`
	TimeoutMS   int64               `json:"timeout_ms"`
}

func (s *Server) handleBulkUserAdd(w http.ResponseWriter, r *http.Request) {
	var req bulkUserAddRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Users) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "total": 0, "added": 0, "failed": 0})
		return
	}

	if _, err := s.requireSession(req.Session); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	report := s.addVouchers(req.Session, req.Concurrency, s.execTimeout(req.TimeoutMS), req.Users)
	report["ok"] = true
	writeJSON(w, http.StatusOK, report)
}

// addVouchers pushes hotspot users onto the router with a bounded worker pool,
// mirroring what the PHP loop did but in parallel.
func (s *Server) addVouchers(session string, concurrency int, timeout time.Duration, vouchers []generator.Voucher) map[string]any {
	if concurrency < 1 {
		concurrency = 16
	}
	if concurrency > 64 {
		concurrency = 64
	}

	cmds := make([][]string, len(vouchers))
	for i, v := range vouchers {
		cmds[i] = []string{
			"/ip/hotspot/user/add",
			"=server=" + v.Server,
			"=name=" + v.Name,
			"=password=" + v.Password,
			"=profile=" + v.Profile,
			"=limit-uptime=" + v.TimeLimit,
			"=limit-bytes-total=" + strconv.FormatInt(v.DataLimit, 10),
			"=comment=" + v.Comment,
		}
	}

	errs, fatal := s.mgr.ExecBatch(session, concurrency, timeout, cmds)
	if fatal != nil {
		return map[string]any{
			"ok":     false,
			"error":  fatal.Error(),
			"total":  len(vouchers),
			"added":  0,
			"failed": len(vouchers),
		}
	}

	added := 0
	var issues []string
	for i, err := range errs {
		if err == nil {
			added++
			continue
		}
		if len(issues) < 25 {
			issues = append(issues, vouchers[i].Name+": "+err.Error())
		}
	}

	return map[string]any{
		"total":      len(vouchers),
		"added":      added,
		"failed":     len(vouchers) - added,
		"first_user": firstName(vouchers),
		"last_user":  lastName(vouchers),
		"errors":     issues,
	}
}

func firstName(v []generator.Voucher) string {
	if len(v) == 0 {
		return ""
	}
	return v[0].Name
}

func lastName(v []generator.Voucher) string {
	if len(v) == 0 {
		return ""
	}
	return v[len(v)-1].Name
}

func clampQty(q int) int {
	if q < 1 {
		return 1
	}
	if q > 100000 {
		return 100000
	}
	return q
}

// ------------------------------------------------------------------ utils

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "POST required"})
		return false
	}
	// PHP sends JSON; a 32 MB cap comfortably fits a 10k-voucher batch.
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write response: %v", err)
	}
}

// requireSession ensures the PHP side is talking about a session this service
// still knows about (it forgets sessions after MIKHMON_API_SESSION_TTL).
func (s *Server) requireSession(id string) (*routeros.Session, error) {
	if id == "" {
		return nil, errors.New("missing session")
	}
	return s.mgr.Session(id)
}
