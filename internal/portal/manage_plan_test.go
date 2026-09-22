package portal

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

/*
 * Paket & harga dulu hanya nilai awal di basis data, jadi tidak ada yang menguji
 * bahwa harganya benar-benar bisa diubah. Tes di berkas ini menjaga dua hal:
 * aturan paketnya, dan janji bahwa mengubah harga langsung terlihat oleh
 * pelanggan di halaman pembayaran (harga diambil dari basis data, bukan dari
 * nilai yang ditanam di kode).
 */

/* ------------------------------------------------------------------ bantu */

// adminHandler menyiapkan server beserta cookie sesi admin yang sah, supaya
// tes bisa memanggil rute admin tanpa melewati proses login.
func adminHandler(t *testing.T, st *Store) (http.Handler, *http.Cookie) {
	t.Helper()
	auth := NewAuth("admin", "rahasia123", "rahasia-uji", false, time.Hour)
	h := NewServer(Config{BaseURL: "http://portal.test", Auth: auth}, st).Handler()

	rec := httptest.NewRecorder()
	auth.SetCookie(rec)
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("cookie sesi admin tidak terbentuk")
	}
	return h, cookies[0]
}

// doJSON memanggil satu rute dan mengembalikan kode balasan beserta isinya.
// Cookie boleh nil untuk rute yang tidak perlu sesi.
func doJSON(t *testing.T, h http.Handler, cookie *http.Cookie, method, path string, body any) (int, map[string]any) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal badan permintaan: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: jawaban bukan JSON: %q", method, path, rec.Body.String())
		}
	}
	return rec.Code, out
}

// planByCode mengambil satu paket dari daftar jawaban /admin/plans.
func planByCode(t *testing.T, body map[string]any, code string) map[string]any {
	t.Helper()
	plans, ok := body["plans"].([]any)
	if !ok {
		t.Fatalf("jawaban tidak punya daftar plans: %v", body)
	}
	for _, raw := range plans {
		p, _ := raw.(map[string]any)
		if p["code"] == code {
			return p
		}
	}
	t.Fatalf("paket %s tidak ada di daftar", code)
	return nil
}

// payTokenFromSeed mengambil token halaman pembayaran pelanggan contoh.
func payTokenFromSeed(t *testing.T, st *Store, session string) string {
	t.Helper()
	var token string
	if err := st.db.QueryRow(`SELECT pay_token FROM customers WHERE session_name = ?`, session).Scan(&token); err != nil {
		t.Fatalf("baca pay_token %s: %v", session, err)
	}
	return token
}

/* -------------------------------------------------------------- ubah harga */

func TestPlanPriceEditable(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	h, cookie := adminHandler(t, st)
	token := payTokenFromSeed(t, st, "taufiq")

	// Harga Awal: P1M = 50000, dipakai satu pelanggan.
	code, body := doJSON(t, h, cookie, "GET", "/api/v1/admin/plans", nil)
	if code != http.StatusOK {
		t.Fatalf("daftar paket: status %d, mau 200 (%v)", code, body)
	}
	if got := planByCode(t, body, "P1M")["price"]; got.(float64) != 50000 {
		t.Fatalf("harga awal P1M = %v, mau 50000", got)
	}
	if got := planByCode(t, body, "P1M")["customers"]; got.(float64) != 1 {
		// Dua pelanggan contoh sama-sama punya pembayaran P1M yang disetujui.
		if got := planByCode(t, body, "P1M")["customers"]; got.(float64) != 2 {
			t.Fatalf("jumlah pelanggan P1M = %v, mau 2", got)
		}
	}

	// Harga diubah menjadi Rp 100.000 per bulan.
	code, body = doJSON(t, h, cookie, "PATCH", "/api/v1/admin/plans/P1M", map[string]any{
		"label": "1 Bulan", "months": 1, "price": 100000, "note": "harga baru",
	})
	if code != http.StatusOK {
		t.Fatalf("ubah harga: status %d, mau 200 (%v)", code, body)
	}
	plan, _ := body["plan"].(map[string]any)
	if plan == nil || plan["price"].(float64) != 100000 {
		t.Fatalf("jawaban ubah harga tidak memuat harga baru: %v", body)
	}

	// Yang penting: halaman pembayaran pelanggan ikut berubah tanpa restart.
	code, body = doJSON(t, h, nil, "GET", "/api/v1/pay/"+token, nil)
	if code != http.StatusOK {
		t.Fatalf("halaman pembayaran: status %d, mau 200 (%v)", code, body)
	}
	plans, _ := body["plans"].([]any)
	found := false
	for _, raw := range plans {
		p, _ := raw.(map[string]any)
		if p["code"] == "P1M" {
			found = true
			if p["price"].(float64) != 100000 {
				t.Errorf("harga di halaman pembayaran = %v, mau 100000", p["price"])
			}
		}
	}
	if !found {
		t.Error("P1M tidak muncul lagi di halaman pembayaran")
	}
}

func TestPlanUpdateKeepsCustomersAndCode(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	h, cookie := adminHandler(t, st)

	// Kode paket yang dikirim di alamat dengan huruf kecil tetap diterima,
	// karena kode selalu dirapikan jadi huruf besar.
	code, body := doJSON(t, h, cookie, "PATCH", "/api/v1/admin/plans/p3m", map[string]any{
		"label": "3 Bulan", "months": 3, "price": 270000, "note": "",
	})
	if code != http.StatusOK {
		t.Fatalf("ubah P3M: status %d, mau 200 (%v)", code, body)
	}

	code, body = doJSON(t, h, cookie, "GET", "/api/v1/admin/plans", nil)
	if code != http.StatusOK {
		t.Fatalf("daftar paket: status %d (%v)", code, body)
	}
	if got := planByCode(t, body, "P3M")["price"]; got.(float64) != 270000 {
		t.Fatalf("harga P3M = %v, mau 270000", got)
	}
	// Pelanggan contoh memakai P1M; P3M tidak boleh ikut berubah pemakainya.
	if got := planByCode(t, body, "P3M")["customers"]; got.(float64) != 0 {
		t.Errorf("jumlah pelanggan P3M = %v, mau 0", got)
	}
}

func TestPlanValidation(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	h, cookie := adminHandler(t, st)

	cases := []struct {
		nama string
		body map[string]any
	}{
		{"harga nol", map[string]any{"label": "1 Bulan", "months": 1, "price": 0, "note": ""}},
		{"harga negatif", map[string]any{"label": "1 Bulan", "months": 1, "price": -5, "note": ""}},
		{"harga keterlaluan", map[string]any{"label": "1 Bulan", "months": 1, "price": 99999999999, "note": ""}},
		{"durasi nol", map[string]any{"label": "1 Bulan", "months": 0, "price": 50000, "note": ""}},
		{"durasi terlalu lama", map[string]any{"label": "1 Bulan", "months": 99, "price": 50000, "note": ""}},
		{"nama kosong", map[string]any{"label": "   ", "months": 1, "price": 50000, "note": ""}},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			code, body := doJSON(t, h, cookie, "PATCH", "/api/v1/admin/plans/P1M", c.body)
			if code != http.StatusBadRequest {
				t.Fatalf("status = %d, mau 400 (%v)", code, body)
			}
			if body["error"] != "invalid" {
				t.Errorf("error = %v, mau invalid", body["error"])
			}
			if msg, _ := body["message"].(string); msg == "" {
				t.Error("pesan galat kosong, admin tidak tahu apa yang salah")
			}
		})
	}

	// Paket yang ditolak tidak boleh berubah harganya.
	_, body := doJSON(t, h, cookie, "GET", "/api/v1/admin/plans", nil)
	if got := planByCode(t, body, "P1M")["price"]; got.(float64) != 50000 {
		t.Fatalf("harga P1M berubah menjadi %v padahal semua masukan ditolak", got)
	}

	// Badan permintaan yang harga/duarasinya bukan angka harus memberi pesan
	// yang bisa dimengerti, bukan kode galat mentah.
	code, body := doJSON(t, h, cookie, "PATCH", "/api/v1/admin/plans/P1M", map[string]any{
		"label": "1 Bulan", "months": "satu", "price": "100000", "note": "",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, mau 400 (%v)", code, body)
	}
	if msg, _ := body["message"].(string); msg == "" {
		t.Errorf("pesan galat kosong: %v", body)
	}
}

func TestPlanUnknownCode(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	h, cookie := adminHandler(t, st)

	code, body := doJSON(t, h, cookie, "PATCH", "/api/v1/admin/plans/TIDAK-ADA", map[string]any{
		"label": "1 Bulan", "months": 1, "price": 100000, "note": "",
	})
	if code != http.StatusNotFound || body["error"] != "unknown_plan" {
		t.Fatalf("status/error = %d/%v, mau 404/unknown_plan", code, body["error"])
	}
}

/* ------------------------------------------------------ tambah hapus paket */

func TestPlanCreateAndDelete(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	h, cookie := adminHandler(t, st)

	code, body := doJSON(t, h, cookie, "POST", "/api/v1/admin/plans", map[string]any{
		"code": "p2m", "label": "2 Bulan", "months": 2, "price": 190000, "note": "hemat",
	})
	if code != http.StatusCreated {
		t.Fatalf("tambah paket: status %d, mau 201 (%v)", code, body)
	}
	plan, _ := body["plan"].(map[string]any)
	if plan == nil || plan["code"] != "P2M" {
		t.Fatalf("kode paket tidak dirapikan jadi huruf besar: %v", body)
	}

	// Kode yang sudah dipakai ditolak, bukan menimpa paket lama.
	code, body = doJSON(t, h, cookie, "POST", "/api/v1/admin/plans", map[string]any{
		"code": "P2M", "label": "2 Bulan Lain", "months": 2, "price": 200000, "note": "",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("tambah kode kembar: status %d, mau 400 (%v)", code, body)
	}

	// Paket baru belum dipakai siapa pun, jadi boleh dihapus.
	code, _ = doJSON(t, h, cookie, "DELETE", "/api/v1/admin/plans/P2M", nil)
	if code != http.StatusNoContent {
		t.Fatalf("hapus paket kosong: status %d, mau 204", code)
	}

	code, body = doJSON(t, h, cookie, "GET", "/api/v1/admin/plans", nil)
	if code != http.StatusOK {
		t.Fatalf("daftar paket: status %d (%v)", code, body)
	}
	for _, raw := range body["plans"].([]any) {
		if p, _ := raw.(map[string]any); p["code"] == "P2M" {
			t.Fatal("paket P2M masih ada setelah dihapus")
		}
	}
}

func TestPlanDeleteRefusedWhileInUse(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	h, cookie := adminHandler(t, st)

	// P1M dipakai pelanggan contoh - penghapusannya harus ditolak dengan
	// pesan yang bisa dibaca admin, bukan galat mentah.
	code, body := doJSON(t, h, cookie, "DELETE", "/api/v1/admin/plans/P1M", nil)
	if code != http.StatusConflict {
		t.Fatalf("status = %d, mau 409 (%v)", code, body)
	}
	if body["error"] != "in_use" {
		t.Errorf("error = %v, mau in_use", body["error"])
	}
	if msg, _ := body["message"].(string); msg == "" {
		t.Error("pesan penolakan kosong")
	}

	// Paketnya harus masih ada dan harga pelanggannya tidak berubah.
	_, body = doJSON(t, h, cookie, "GET", "/api/v1/admin/plans", nil)
	if got := planByCode(t, body, "P1M")["price"]; got.(float64) != 50000 {
		t.Fatalf("harga P1M = %v, mau tetap 50000", got)
	}
}

/* -------------------------------------------------------------------- sesi */

func TestPlanRoutesNeedAdminSession(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	h, _ := adminHandler(t, st)

	rutes := []struct {
		method, path string
	}{
		{"GET", "/api/v1/admin/plans"},
		{"POST", "/api/v1/admin/plans"},
		{"PATCH", "/api/v1/admin/plans/P1M"},
		{"DELETE", "/api/v1/admin/plans/P1M"},
	}
	for _, rt := range rutes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			code, body := doJSON(t, h, nil, rt.method, rt.path, nil)
			if code != http.StatusUnauthorized || body["error"] != "unauthorized" {
				t.Fatalf("status/error = %d/%v, mau 401/unauthorized", code, body["error"])
			}
		})
	}
}

/* ------------------------------------------------------------ lapisan data */

func TestPlanStoreRules(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// Kode paket dirapikan: spasi dan huruf kecil tidak diteruskan apa adanya.
	if got := NormalizePlanCode(" p2 m "); got != "P2-M" {
		t.Errorf("NormalizePlanCode = %q, mau P2-M", got)
	}
	if got := NormalizePlanCode("!!"); got != "" {
		t.Errorf("NormalizePlanCode = %q, mau kosong", got)
	}

	// Paket baru ditaruh paling bawah supaya urutan di halaman pembayaran tetap.
	tambahan, err := st.SavePlan(PlanInput{Code: "P24M", Label: "24 Bulan", Months: 24, Price: 800000})
	if err != nil {
		t.Fatalf("SavePlan: %v", err)
	}
	semua, err := st.Plans(true)
	if err != nil {
		t.Fatalf("Plans: %v", err)
	}
	if semua[len(semua)-1].Code != tambahan.Code {
		t.Errorf("paket baru tidak di urutan terakhir: %v", semua)
	}

	// Menyimpan ulang kode yang sama harus ditolak, bukan menimpa.
	if _, err := st.SavePlan(PlanInput{Code: "P24M", Label: "Duplikat", Months: 1, Price: 1000}); err == nil {
		t.Error("SavePlan menerima kode paket yang sudah ada")
	}

	// Paket yang dipakai pelanggan tidak boleh hilang.
	if err := st.DeletePlan("P1M"); err == nil {
		t.Error("DeletePlan menghapus paket yang masih dipakai pelanggan")
	}
	if _, err := st.PlanByCode("P1M"); err != nil {
		t.Errorf("P1M hilang setelah penghapusan ditolak: %v", err)
	}

	// PlanUsage menghitung pemakai tiap paket.
	usage, err := st.PlanUsage()
	if err != nil {
		t.Fatalf("PlanUsage: %v", err)
	}
	// Dua pelanggan contoh sama-sama punya pembayaran P1M yang disetujui.
	if usage["P1M"] != 2 {
		t.Errorf("PlanUsage[P1M] = %d, mau 2", usage["P1M"])
	}
	if usage["P24M"] != 0 {
		t.Errorf("PlanUsage[P24M] = %d, mau 0", usage["P24M"])
	}
}
