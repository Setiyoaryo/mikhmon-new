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

func (s *Server) handlePlans(w http.ResponseWriter, r *http.Request) {
	plans, err := s.store.Plans(true)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	if plans == nil {
		plans = []Plan{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans})
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
