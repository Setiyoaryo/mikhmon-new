package portal

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

/*
 * Endpoint pengelolaan pelanggan untuk halaman admin portal.
 *
 * Semua di bawah /api/v1/admin dan wajib login. Galat masukan dibalas 400
 * dengan bentuk {"error":"invalid","message":"..."} supaya antarmuka bisa
 * menampilkan alasannya apa adanya, bukan pesan generik.
 */

// adminCustomerRow menyusun satu baris pelanggan seperti yang dipakai
// halaman admin. Dipakai oleh ringkasan maupun endpoint kelola.
func (s *Server) adminCustomerRow(c Customer) adminCustomer {
	sub := s.subscription(c)
	inst, _ := s.store.InstanceByID(c.InstanceID)

	row := adminCustomer{
		ID:          c.ID,
		Name:        c.Name,
		Institution: c.Institution,
		InstanceID:  inst.ID,
		SessionName: c.SessionName,
		PlanCode:    sub.PlanCode,
		PlanLabel:   sub.PlanLabel,
		DaysLeft:    sub.DaysLeft,
		Status:      sub.State,
		WA:          c.WA,
		LastSeen:    humanAgo(inst.LastSeen),
		PayURL:      strings.TrimRight(s.cfg.BaseURL, "/") + "/#/pay/" + c.PayToken,
	}
	if p, err := s.store.PlanByCode(sub.PlanCode); err == nil && p.Months > 0 {
		row.Monthly = p.Price / int64(p.Months)
	}
	return row
}

// writeInvalid membalas galat masukan beserta alasannya.
func writeInvalid(w http.ResponseWriter, err error) {
	var bad ErrInvalid
	if errors.As(err, &bad) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid",
			"message": bad.Msg,
		})
		return
	}
	log.Printf("galat kelola pelanggan: %v", err)
	writeErr(w, http.StatusInternalServerError, "store_error")
}

/* ------------------------------------------------------------------ paket */

// planJSON menyiapkan satu paket untuk jawaban API. Jumlah pelanggan
// pemakainya ikut dikirim supaya halaman admin bisa memperingatkan sebelum
// paket dihapus, bukan sesudah ditolak server.
func planJSON(p Plan, customers int) map[string]any {
	return map[string]any{
		"code":      p.Code,
		"label":     p.Label,
		"months":    p.Months,
		"price":     p.Price,
		"note":      p.Note,
		"customers": customers,
	}
}

/*
 * handlePlans mengembalikan paket yang dijual beserta harga terkininya.
 * Harga diambil dari basis data setiap kali dipanggil - tidak ada nilai harga
 * yang ditanam di kode - sehingga perubahan harga langsung terlihat di halaman
 * pembayaran pelanggan.
 */
func (s *Server) handlePlans(w http.ResponseWriter, r *http.Request) {
	plans, err := s.store.Plans(false)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	usage, err := s.store.PlanUsage()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}

	out := make([]map[string]any, 0, len(plans))
	for _, p := range plans {
		out = append(out, planJSON(p, usage[p.Code]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": out})
}

// writePlanErr menerjemahkan galat paket jadi kode balasan yang tepat.
func writePlanErr(w http.ResponseWriter, err error) {
	var inUse ErrInUse
	if errors.As(err, &inUse) {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error":   "in_use",
			"message": inUse.Msg,
		})
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "unknown_plan")
		return
	}
	writeInvalid(w, err)
}

// pesanPaketTidakTerbaca dipakai saat isi permintaan bukan JSON yang sesuai,
// supaya admin melihat sebabnya alih-alih kode "bad_request" mentah.
var errPaketTidakTerbaca = invalid("Data paket tidak terbaca. " +
	"Pastikan harga dan durasi diisi berupa angka.")

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code   string `json:"code"`
		Label  string `json:"label"`
		Months int    `json:"months"`
		Price  int64  `json:"price"`
		Note   string `json:"note"`
	}
	if err := decode(r, &req); err != nil {
		writeInvalid(w, errPaketTidakTerbaca)
		return
	}

	plan, err := s.store.SavePlan(PlanInput{
		Code: req.Code, Label: req.Label, Months: req.Months, Price: req.Price, Note: req.Note,
	})
	if err != nil {
		writePlanErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "plan": planJSON(plan, 0)})
}

// handleUpdatePlan mengubah paket yang sudah ada. Kode paket ada di alamat,
// bukan di badan permintaan, karena kode itu tidak bisa diganti.
func (s *Server) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label  string `json:"label"`
		Months int    `json:"months"`
		Price  int64  `json:"price"`
		Note   string `json:"note"`
	}
	if err := decode(r, &req); err != nil {
		writeInvalid(w, errPaketTidakTerbaca)
		return
	}

	code := r.PathValue("code")
	plan, err := s.store.UpdatePlan(code, PlanInput{
		Label: req.Label, Months: req.Months, Price: req.Price, Note: req.Note,
	})
	if err != nil {
		writePlanErr(w, err)
		return
	}

	usage, err := s.store.PlanUsage()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"plan": planJSON(plan, usage[plan.Code]),
	})
}

func (s *Server) handleDeletePlan(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeletePlan(r.PathValue("code")); err != nil {
		writePlanErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

/* ------------------------------------------------------------- pemasangan */

func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Instances()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	if list == nil {
		list = []InstanceUsage{}
	}
	out := make([]map[string]any, 0, len(list))
	for _, u := range list {
		out = append(out, map[string]any{
			"id":          u.ID,
			"token":       u.Token,
			"name":        u.Name,
			"kind":        u.Kind,
			"router_name": u.RouterName,
			"version":     u.Version,
			"last_seen":   humanAgo(u.LastSeen),
			"customers":   u.Customers,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"instances": out})
}

func (s *Server) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	inst, err := s.store.CreateInstance(req.Name, req.Kind)
	if err != nil {
		writeInvalid(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok": true,
		"instance": map[string]any{
			"id":        inst.ID,
			"token":     inst.Token,
			"name":      inst.Name,
			"kind":      inst.Kind,
			"customers": 0,
		},
	})
}

/* -------------------------------------------------------------- pelanggan */

func (s *Server) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Institution string `json:"institution"`
		WA          string `json:"wa"`
		SessionName string `json:"session_name"`
		InstanceID  string `json:"instance_id"`
		PlanCode    string `json:"plan_code"`
		ValidUntil  string `json:"valid_until"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	cust, err := s.store.CreateCustomer(NewCustomer{
		Name:        req.Name,
		Institution: req.Institution,
		WA:          req.WA,
		SessionName: req.SessionName,
		InstanceID:  req.InstanceID,
		PlanCode:    req.PlanCode,
		ValidUntil:  req.ValidUntil,
	})
	if err != nil {
		writeInvalid(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":       true,
		"customer": s.adminCustomerRow(cust),
	})
}

func (s *Server) handleUpdateCustomer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Institution string `json:"institution"`
		WA          string `json:"wa"`
		SessionName string `json:"session_name"`
		InstanceID  string `json:"instance_id"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	id := r.PathValue("id")
	if err := s.store.UpdateCustomer(id, CustomerEdit{
		Name:        req.Name,
		Institution: req.Institution,
		WA:          req.WA,
		SessionName: req.SessionName,
		InstanceID:  req.InstanceID,
	}); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeInvalid(w, err)
		return
	}
	cust, err := s.store.CustomerByID(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"customer": s.adminCustomerRow(cust),
	})
}

func (s *Server) handleExtend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Months int `json:"months"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	until, err := s.store.ExtendCustomer(r.PathValue("id"), req.Months)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeInvalid(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"expires_at": until.Format("2006-01-02"),
	})
}

func (s *Server) handleDeleteCustomer(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteCustomer(r.PathValue("id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// parseMonths dipakai kalau lama perpanjangan dikirim sebagai teks.
func parseMonths(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0
	}
	return n
}

func (s *Server) handleDeleteInstance(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteInstance(r.PathValue("id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found")
			return
		}
		writeInvalid(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

/*
 * handleEnroll mendaftarkan panel yang baru dipasang.
 *
 * Sengaja tanpa login: panelnya sendiri yang melapor saat pertama kali
 * dijalankan, supaya tidak ada berkas berisi id dan token yang harus diisi
 * tangan di tiap VPS pelanggan. Yang dihasilkan cuma sebuah identitas panel -
 * tidak bisa dipakai untuk melihat atau mengubah apa pun kecuali melapor
 * statusnya sendiri, dan pemilik portal bisa menghapusnya dari halaman admin.
 */
func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Enroll {
		writeErr(w, http.StatusForbidden, "enroll_disabled")
		return
	}
	var req struct {
		Host    string `json:"host"`
		Version string `json:"version"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	inst, err := s.store.EnrollInstance(req.Host, req.Version)
	if err != nil {
		log.Printf("gagal mendaftarkan panel: %v", err)
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok": true,
		"instance": map[string]any{
			"id":    inst.ID,
			"token": inst.Token,
			"name":  inst.Name,
		},
	})
}
