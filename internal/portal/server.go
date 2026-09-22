package portal

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// web memuat hasil build frontend Svelte. Isinya disalin ke sini waktu build
// Docker; kalau kosong, server menjelaskan cara mengisinya.
//
//go:embed all:web
var web embed.FS

// Config adalah pengaturan server portal.
type Config struct {
	Addr      string
	DBPath    string
	WebDir    string // kalau diisi, dipakai sebagai ganti yang di-embed
	BaseURL   string
	QRISImage string
	// Enroll menentukan apakah panel baru boleh mendaftarkan dirinya sendiri.
	// Dimatikan kalau portalnya dipakai untuk lebih dari satu penyewa.
	Enroll       bool
	QRISMerchant string
	QRISNMID     string
	WANumber     string
	Auth         *Auth
}

// Server melayani API portal dan halaman pembayaran.
type Server struct {
	cfg   Config
	store *Store
	mux   *http.ServeMux
}

// NewServer menyiapkan semua rute.
func NewServer(cfg Config, store *Store) *Server {
	s := &Server{cfg: cfg, store: store, mux: http.NewServeMux()}

	// --- panel Mikhmon ---
	s.mux.HandleFunc("POST /api/v1/heartbeat", s.handleHeartbeat)
	s.mux.HandleFunc("POST /api/v1/enroll", s.handleEnroll)

	// --- halaman pelanggan ---
	s.mux.HandleFunc("GET /api/v1/pay/{token}", s.handlePay)
	s.mux.HandleFunc("POST /api/v1/pay/{token}/claim", s.handleClaim)

	// --- admin ---
	s.mux.HandleFunc("POST /api/v1/admin/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/admin/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/v1/admin/me", s.admin(s.handleMe))
	s.mux.HandleFunc("GET /api/v1/admin/overview", s.admin(s.handleOverview))
	s.mux.HandleFunc("POST /api/v1/admin/claims/{id}/approve", s.admin(s.handleApprove))
	s.mux.HandleFunc("POST /api/v1/admin/claims/{id}/reject", s.admin(s.handleReject))
	s.mux.HandleFunc("POST /api/v1/admin/customers/{id}/suspend", s.admin(s.handleSuspend))
	s.mux.HandleFunc("POST /api/v1/admin/customers/{id}/activate", s.admin(s.handleActivate))

	// --- admin: kelola pelanggan ---
	s.mux.HandleFunc("GET /api/v1/admin/plans", s.admin(s.handlePlans))
	// Paket & harga: dulu hanya nilai awal di basis data, sekarang bisa diubah
	// dari halaman admin tanpa menyentuh kode.
	s.mux.HandleFunc("POST /api/v1/admin/plans", s.admin(s.handleCreatePlan))
	s.mux.HandleFunc("PATCH /api/v1/admin/plans/{code}", s.admin(s.handleUpdatePlan))
	s.mux.HandleFunc("DELETE /api/v1/admin/plans/{code}", s.admin(s.handleDeletePlan))
	s.mux.HandleFunc("GET /api/v1/admin/instances", s.admin(s.handleInstances))
	s.mux.HandleFunc("POST /api/v1/admin/instances", s.admin(s.handleCreateInstance))
	s.mux.HandleFunc("DELETE /api/v1/admin/instances/{id}", s.admin(s.handleDeleteInstance))
	s.mux.HandleFunc("POST /api/v1/admin/customers", s.admin(s.handleCreateCustomer))
	s.mux.HandleFunc("PATCH /api/v1/admin/customers/{id}", s.admin(s.handleUpdateCustomer))
	s.mux.HandleFunc("POST /api/v1/admin/customers/{id}/extend", s.admin(s.handleExtend))
	s.mux.HandleFunc("DELETE /api/v1/admin/customers/{id}", s.admin(s.handleDeleteCustomer))

	// --- lain-lain ---
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "ok")
	})
	// Gambar QRIS: alamatnya tetap /qris.png, tetapi berkasnya bisa berubah
	// ekstensi mengikuti yang terakhir diunggah dari halaman admin.
	s.mux.HandleFunc("GET /qris.png", s.handleQRISImage)
	s.mux.HandleFunc("POST /api/v1/admin/qris", s.admin(s.handleQRISUpload))
	s.mux.HandleFunc("DELETE /api/v1/admin/qris", s.admin(s.handleQRISDelete))
	s.mux.HandleFunc("/", s.handleWeb)

	return s
}

// Handler mengembalikan penangan HTTP untuk dipasang di server.
func (s *Server) Handler() http.Handler {
	return requestLogger(s.mux)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
			return
		}
		next.ServeHTTP(w, r)
	})
}

/* ------------------------------------------------------------------ util */

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("gagal menulis balasan JSON: %v", err)
	}
}

func writeErr(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(v)
}

func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.Auth.LoggedIn(r) {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

/* -------------------------------------------------------------- langganan */

type subState struct {
	State     string `json:"state"`
	PlanCode  string `json:"plan_code"`
	PlanLabel string `json:"plan_label"`
	StartedAt string `json:"started_at"`
	ExpiresAt string `json:"expires_at"`
	DaysLeft  int    `json:"days_left"`
}

// statusOf menerjemahkan sisa hari jadi keadaan yang dipakai antarmuka.
func statusOf(daysLeft int) string {
	switch {
	case daysLeft < -3:
		return "expired"
	case daysLeft < 0:
		return "grace"
	case daysLeft <= 7:
		return "warning"
	default:
		return "active"
	}
}

// subscription menghitung keadaan langganan seorang pelanggan.
func (s *Server) subscription(c Customer) subState {
	out := subState{
		PlanCode:  s.lastPlanCode(c.ID),
		StartedAt: c.ValidFrom,
		ExpiresAt: c.ValidUntil,
	}
	if p, err := s.store.PlanByCode(out.PlanCode); err == nil {
		out.PlanLabel = p.Label
	}
	if strings.TrimSpace(c.ValidUntil) == "" {
		out.State = "expired"
		return out
	}
	until, err := time.ParseInLocation("2006-01-02", c.ValidUntil, loc)
	if err != nil {
		out.State = "expired"
		return out
	}
	out.DaysLeft = int(until.Sub(today()).Hours() / 24)
	// Pembulatan jam bisa meleset satu hari karena DST di zona lain; disamakan
	// dengan selisih tanggal kalender supaya cocok dengan yang dilihat pelanggan.
	if out.DaysLeft >= 0 {
		out.DaysLeft = int(until.Sub(today()).Round(24*time.Hour).Hours() / 24)
	}
	// Ditangguhkan tetap dihitung sisa harinya, supaya panel bisa menampilkan
	// "berakhir 26 Des" sekaligus "dihentikan" tanpa angka yang menyesatkan.
	if c.Suspended {
		out.State = "expired"
		return out
	}
	out.State = statusOf(out.DaysLeft)
	return out
}

// lastPlanCode mencari paket dari pembayaran terakhir yang disetujui.
func (s *Server) lastPlanCode(customerID string) string {
	var code string
	err := s.store.db.QueryRow(
		`SELECT plan_code FROM payments WHERE customer_id = ? AND status = 'approved'
		 ORDER BY decided_at DESC LIMIT 1`, customerID).Scan(&code)
	if err != nil || code == "" {
		// Belum pernah bayar: pakai paket cadangan yang sama dengan yang
		// dihitung PlanUsage, supaya keduanya tidak bisa berbeda.
		return fallbackPlanCode(s.store.db)
	}
	return code
}

/* -------------------------------------------------------------- heartbeat */

type heartbeatReq struct {
	InstanceID string `json:"instance_id"`
	Token      string `json:"token"`
	Version    string `json:"version"`
	// Tenant adalah label subdomain pelanggan di deployment bersama, mis.
	// "taufiq". Panel khusus yang hanya melayani satu pelanggan boleh
	// mengosongkannya.
	Tenant string `json:"tenant"`
	// Host adalah alamat web panelnya sendiri, mis. "taufiq.nocify.id". Hanya
	// dipakai untuk melengkapi nama panel yang masih nama bawaan; boleh kosong.
	Host string `json:"host"`
}

// heartbeatResp adalah satu-satunya hal yang perlu diketahui panel pelanggan.
type heartbeatResp struct {
	State      string `json:"state"`
	Plan       string `json:"plan"`
	ExpiresAt  string `json:"expires_at"`
	DaysLeft   int    `json:"days_left"`
	PayURL     string `json:"pay_url"`
	Message    string `json:"message"`
	ServerTime string `json:"server_time"`
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req heartbeatReq
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	inst, err := s.store.InstanceByID(strings.TrimSpace(req.InstanceID))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "unknown_instance")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	if inst.Token != req.Token {
		writeErr(w, http.StatusUnauthorized, "bad_token")
		return
	}
	// Cari pelanggan yang dimaksud. Deployment bersama mengirim "tenant"
	// (label subdomain), deployment khusus yang cuma melayani satu pelanggan
	// boleh tidak mengirim apa pun.
	tenant := strings.TrimSpace(req.Tenant)
	var cust Customer
	if tenant != "" {
		cust, err = s.store.CustomerBySession(inst.ID, tenant)
		if errors.Is(err, ErrNotFound) {
			writeErr(w, http.StatusNotFound, "unknown_tenant")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "store_error")
			return
		}
	} else {
		hosted, herr := s.store.CustomersByInstance(inst.ID)
		if herr != nil {
			writeErr(w, http.StatusInternalServerError, "store_error")
			return
		}
		switch {
		case len(hosted) == 0:
			writeErr(w, http.StatusNotFound, "unknown_instance")
			return
		case len(hosted) > 1:
			// Deployment bersama tapi panelnya tidak menyebut pelanggan mana.
			writeErr(w, http.StatusBadRequest, "tenant_required")
			return
		}
		cust = hosted[0]
	}
	// Panel yang mendaftar tanpa alamat web (mis. dijalankan dari baris
	// perintah) namanya jadi "Panel tanpa nama". Laporan berikutnya datang
	// dari browser, jadi alamatnya bisa dipakai memperbaiki nama itu.
	if _, err := s.store.NameInstance(inst.ID, req.Host); err != nil && !errors.Is(err, ErrNotFound) {
		log.Printf("galat melengkapi nama panel %s: %v", inst.ID, err)
	}
	_ = s.store.TouchInstance(inst.ID, req.Version)

	sub := s.subscription(cust)
	resp := heartbeatResp{
		State:      sub.State,
		Plan:       sub.PlanLabel,
		ExpiresAt:  sub.ExpiresAt,
		DaysLeft:   sub.DaysLeft,
		PayURL:     strings.TrimRight(s.cfg.BaseURL, "/") + "/#/pay/" + cust.PayToken,
		ServerTime: Now().Format(time.RFC3339),
	}
	switch sub.State {
	case "active":
		resp.Message = fmt.Sprintf("Langganan aktif sampai %s.", sub.ExpiresAt)
	case "warning":
		resp.Message = fmt.Sprintf("Langganan berakhir dalam %d hari (%s).", sub.DaysLeft, sub.ExpiresAt)
	case "grace":
		resp.Message = fmt.Sprintf("Langganan sudah lewat %d hari. Segera diperpanjang.", -sub.DaysLeft)
	default:
		if cust.Suspended {
			resp.Message = "Langganan dihentikan. Hubungi NOCIFY."
		} else {
			resp.Message = "Langganan berakhir. Perpanjang untuk membuka panel."
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

/* ------------------------------------------------------ halaman pembayaran */

type claimView struct {
	Ref       string `json:"ref"`
	PlanCode  string `json:"plan_code"`
	PlanLabel string `json:"plan_label"`
	Amount    int64  `json:"amount"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) claimView(c Claim) claimView {
	v := claimView{Ref: c.Ref, PlanCode: c.PlanCode, Amount: c.Amount, CreatedAt: c.CreatedAt}
	if p, err := s.store.PlanByCode(c.PlanCode); err == nil {
		v.PlanLabel = p.Label
	}
	return v
}

func (s *Server) handlePay(w http.ResponseWriter, r *http.Request) {
	cust, err := s.store.CustomerByPayToken(r.PathValue("token"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}

	plans, err := s.store.Plans(true)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	if plans == nil {
		plans = []Plan{}
	}

	inst, _ := s.store.InstanceByCustomer(cust.ID)

	var pending any
	if c, err := s.store.PendingClaim(cust.ID); err == nil {
		pending = s.claimView(c)
	}

	imageURL := any(nil)
	if u := s.qrisURL(); u != "" {
		imageURL = u
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"customer": map[string]string{"name": cust.Name, "institution": cust.Institution},
		"instance": map[string]string{
			"id":          inst.ID,
			"router_name": inst.RouterName,
			"version":     inst.Version,
		},
		"subscription": s.subscription(cust),
		"plans":        plans,
		"qris": map[string]any{
			"merchant":  s.cfg.QRISMerchant,
			"nmid":      s.cfg.QRISNMID,
			"image_url": imageURL,
		},
		"wa_number":     s.cfg.WANumber,
		"pending_claim": pending,
	})
}

func (s *Server) handleClaim(w http.ResponseWriter, r *http.Request) {
	cust, err := s.store.CustomerByPayToken(r.PathValue("token"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}

	var req struct {
		PlanCode string `json:"plan_code"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}

	claim, err := s.store.CreateClaim(cust.ID, strings.TrimSpace(req.PlanCode))
	switch {
	case errors.Is(err, ErrPending):
		// Sudah ada klaim yang menunggu: balikkan yang itu, jangan bikin baru.
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "already_pending",
			"claim": s.claimView(claim),
		})
		return
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusBadRequest, "unknown_plan")
		return
	case err != nil:
		log.Printf("gagal membuat klaim: %v", err)
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}

	_ = s.store.LogEvent(fmt.Sprintf("%s mengirim klaim pembayaran %s.", cust.Institution, claim.Ref))
	writeJSON(w, http.StatusOK, s.claimView(claim))
}

func fileExists(p string) bool {
	if strings.TrimSpace(p) == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

/* ------------------------------------------------------------------ admin */

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	if !s.cfg.Auth.Authenticate(req.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	s.cfg.Auth.SetCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.cfg.Auth.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"user": s.cfg.Auth.User()})
}

type adminClaim struct {
	ID          string `json:"id"`
	Customer    string `json:"customer"`
	Institution string `json:"institution"`
	PlanCode    string `json:"plan_code"`
	PlanLabel   string `json:"plan_label"`
	Amount      int64  `json:"amount"`
	Months      int    `json:"months"`
	Ref         string `json:"ref"`
	CreatedAt   string `json:"created_at"`
	Age         string `json:"age"`
}

type adminCustomer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Institution string `json:"institution"`
	InstanceID  string `json:"instance_id"`
	SessionName string `json:"session_name"`
	PlanCode    string `json:"plan_code"`
	PlanLabel   string `json:"plan_label"`
	DaysLeft    int    `json:"days_left"`
	Status      string `json:"status"`
	LastSeen    string `json:"last_seen"`
	Monthly     int64  `json:"monthly"`
	WA          string `json:"wa"`
	PayURL      string `json:"pay_url"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	customers, err := s.store.AllCustomers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	claims, err := s.store.PendingClaims()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	revenue, err := s.store.RevenueThisMonth()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}

	outClaims := make([]adminClaim, 0, len(claims))
	for _, c := range claims {
		v := adminClaim{
			ID: c.ID, Customer: c.Customer, Institution: c.Institution,
			PlanCode: c.PlanCode, Amount: c.Amount, Months: c.Months, Ref: c.Ref,
			CreatedAt: c.CreatedAt, Age: humanAgo(c.CreatedAt),
		}
		if p, err := s.store.PlanByCode(c.PlanCode); err == nil {
			v.PlanLabel = p.Label
		}
		outClaims = append(outClaims, v)
	}

	outCustomers := make([]adminCustomer, 0, len(customers))
	active, warning, expired := 0, 0, 0
	for _, c := range customers {
		row := s.adminCustomerRow(c)
		switch row.Status {
		case "active":
			active++
		case "warning", "grace":
			warning++
		default:
			expired++
		}
		outCustomers = append(outCustomers, row)
	}

	events, err := s.store.RecentEvents(6)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	activity := make([]map[string]string, 0, len(events))
	for _, e := range events {
		activity = append(activity, map[string]string{"at": humanAgo(e.At), "text": e.Text})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"stats": map[string]any{
			"revenue_month": revenue,
			"customers":     len(customers),
			"active":        active,
			"warning":       warning,
			"expired":       expired,
		},
		"claims":    outClaims,
		"customers": outCustomers,
		"activity":  activity,
		"qris_url":  s.qrisURL(),
	})
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	until, err := s.store.ApproveClaim(r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		log.Printf("gagal menyetujui klaim: %v", err)
		writeErr(w, http.StatusConflict, "not_pending")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"expires_at": until.Format(time.RFC3339),
	})
}

func (s *Server) handleReject(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RejectClaim(r.PathValue("id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeErr(w, http.StatusConflict, "not_pending")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSuspend(w http.ResponseWriter, r *http.Request) {
	s.setSuspended(w, r, true)
}

func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request) {
	s.setSuspended(w, r, false)
}

func (s *Server) setSuspended(w http.ResponseWriter, r *http.Request, suspended bool) {
	id := r.PathValue("id")
	if err := s.store.SetSuspended(id, suspended); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	cust, err := s.store.CustomerByID(id)
	if err == nil {
		if suspended {
			_ = s.store.LogEvent(cust.Institution + " ditangguhkan.")
		} else {
			_ = s.store.LogEvent(cust.Institution + " diaktifkan kembali.")
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

/* ---------------------------------------------------------- berkas statis */

func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	h, err := s.webHandler()
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "Antarmuka portal belum di-build.\n\n"+
			"Jalankan: cd portal && npm install && npm run build\n"+
			"lalu salin portal/dist ke internal/portal/web (dilakukan otomatis oleh Dockerfile.portal),\n"+
			"atau set MIKHMON_PORTAL_WEB_DIR ke folder hasil build.\n")
		return
	}
	h.ServeHTTP(w, r)
}

func (s *Server) webHandler() (http.Handler, error) {
	if dir := strings.TrimSpace(s.cfg.WebDir); dir != "" {
		return spaHandler(os.DirFS(dir)), nil
	}
	sub, err := fs.Sub(web, "web")
	if err != nil {
		return nil, err
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, err
	}
	return spaHandler(sub), nil
}

// spaHandler menyajikan berkas statis dan mengembalikan index.html untuk rute
// yang tidak dikenal, karena aplikasinya satu halaman.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" || p == "." {
			p = "index.html"
		}
		if _, err := fs.Stat(fsys, p); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
			if _, err := fs.Stat(fsys, "index.html"); err != nil {
				http.NotFound(w, r)
				return
			}
			serveIndex(w, r, fsys)
			return
		}
		if strings.HasPrefix(p, "assets/") || strings.HasPrefix(p, "font-awesome/") {
			w.Header().Set("Cache-Control", "public, max-age=604800")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	b, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

/* ------------------------------------------------------------------ waktu */

// humanAgo mengubah waktu RFC3339 jadi "6 menit lalu" untuk ditampilkan.
// Kosong atau tidak terbaca menghasilkan "belum pernah".
func humanAgo(iso string) string {
	if strings.TrimSpace(iso) == "" {
		return "belum pernah"
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return "belum pernah"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "baru saja"
	case d < time.Hour:
		return fmt.Sprintf("%d menit lalu", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d jam lalu", int(d.Hours()))
	default:
		return fmt.Sprintf("%d hari lalu", int(d.Hours()/24))
	}
}
