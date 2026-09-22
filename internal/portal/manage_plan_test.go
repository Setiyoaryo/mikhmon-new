package portal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// customerFromSeed mengambil pelanggan contoh dari label sesinya.
func customerFromSeed(t *testing.T, st *Store, session string) Customer {
	t.Helper()
	var id string
	if err := st.db.QueryRow(`SELECT id FROM customers WHERE session_name = ?`, session).Scan(&id); err != nil {
		t.Fatalf("baca pelanggan %s: %v", session, err)
	}
	c, err := st.CustomerByID(id)
	if err != nil {
		t.Fatalf("CustomerByID(%s): %v", session, err)
	}
	return c
}

// mustDate membaca tanggal YYYY-MM-DD yang sudah ada di basis data.
func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		t.Fatalf("tanggal %q tidak terbaca: %v", s, err)
	}
	return d
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
	// Dua pelanggan contoh sama-sama punya pembayaran P1M yang disetujui.
	if got := planByCode(t, body, "P1M")["customers"]; got.(float64) != 2 {
		t.Fatalf("jumlah pelanggan P1M = %v, mau 2", got)
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
	// P24M baru dibuat dan belum dipakai siapa pun: kuncinya tidak muncul di
	// peta pemakaian, bukan muncul dengan nilai 0.
	if n, ada := usage["P24M"]; ada || n != 0 {
		t.Errorf("PlanUsage[P24M] = %d (ada = %v), mau tidak ada pemakai", n, ada)
	}
}

/* ------------------------------------------------- durasi klaim & hapus paket */

// TestClaimDurationFrozen mengunci janji perbaikan durasi: bulan yang dibeli
// dibekukan saat klaim dibuat, jadi mengubah paket setelahnya (mis. P1M jadi
// 12 bulan) tidak menambah masa berlaku pelanggan.
func TestClaimDurationFrozen(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	cust := customerFromSeed(t, st, "taufiq")

	claim, err := st.CreateClaim(cust.ID, "P1M")
	if err != nil {
		t.Fatalf("CreateClaim: %v", err)
	}
	if claim.Months != 1 {
		t.Fatalf("claim.Months = %d, mau 1 (durasi saat klaim dibuat)", claim.Months)
	}

	// Admin mengubah P1M menjadi paket 12 bulan setelah klaim dibuat.
	if _, err := st.UpdatePlan("P1M", PlanInput{Label: "1 Bulan", Months: 12, Price: 50000}); err != nil {
		t.Fatalf("UpdatePlan: %v", err)
	}

	until, err := st.ApproveClaim(claim.ID)
	if err != nil {
		t.Fatalf("ApproveClaim: %v", err)
	}
	want := addMonths(mustDate(t, cust.ValidUntil), 1)
	if !until.Equal(want) {
		t.Fatalf("berlaku sampai %s, mau %s (1 bulan yang dibeli, bukan 12)",
			until.Format("2006-01-02"), want.Format("2006-01-02"))
	}
	after, err := st.CustomerByID(cust.ID)
	if err != nil {
		t.Fatalf("CustomerByID: %v", err)
	}
	if after.ValidUntil != want.Format("2006-01-02") {
		t.Errorf("valid_until pelanggan = %s, mau %s", after.ValidUntil, want.Format("2006-01-02"))
	}
}

// TestApproveClaimWithoutPlan menjaga klaim yang paketnya terhapus: harga dan
// durasinya sudah tersimpan di klaim, jadi masa berlakunya tetap bisa dihitung.
func TestApproveClaimWithoutPlan(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	cust := customerFromSeed(t, st, "taufiq")

	claim, err := st.CreateClaim(cust.ID, "P3M")
	if err != nil {
		t.Fatalf("CreateClaim: %v", err)
	}
	// Dihapus langsung lewat basis data: DeletePlan memang menolak selama
	// klaimnya masih menunggu, dan yang diuji di sini jalur pemulihannya.
	if _, err := st.db.Exec(`DELETE FROM plans WHERE code = 'P3M'`); err != nil {
		t.Fatalf("hapus paket P3M: %v", err)
	}

	until, err := st.ApproveClaim(claim.ID)
	if err != nil {
		t.Fatalf("ApproveClaim tanpa paket: %v", err)
	}
	want := addMonths(mustDate(t, cust.ValidUntil), 3)
	if !until.Equal(want) {
		t.Fatalf("berlaku sampai %s, mau %s (3 bulan yang dibeli)",
			until.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

func TestPlanDeleteRefusedWhileClaimPending(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	h, cookie := adminHandler(t, st)
	cust := customerFromSeed(t, st, "taufiq")

	if _, err := st.SavePlan(PlanInput{Code: "P24M", Label: "24 Bulan", Months: 24, Price: 800000}); err != nil {
		t.Fatalf("SavePlan: %v", err)
	}
	if _, err := st.CreateClaim(cust.ID, "P24M"); err != nil {
		t.Fatalf("CreateClaim: %v", err)
	}

	code, body := doJSON(t, h, cookie, "DELETE", "/api/v1/admin/plans/P24M", nil)
	if code != http.StatusConflict {
		t.Fatalf("status = %d, mau 409 (%v)", code, body)
	}
	if body["error"] != "in_use" {
		t.Errorf("error = %v, mau in_use", body["error"])
	}
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "diputus") {
		t.Errorf("pesan %q harus menyebut pembayarannya harus diputus dulu", msg)
	}
	if _, err := st.PlanByCode("P24M"); err != nil {
		t.Errorf("paket P24M terhapus padahal masih menunggu klaim: %v", err)
	}
}

func TestPlanDeleteRefusedForLastPlan(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.SavePlan(PlanInput{Code: "A1", Label: "Paket A", Months: 1, Price: 10000}); err != nil {
		t.Fatalf("SavePlan A1: %v", err)
	}
	if _, err := st.SavePlan(PlanInput{Code: "B1", Label: "Paket B", Months: 1, Price: 20000}); err != nil {
		t.Fatalf("SavePlan B1: %v", err)
	}

	if err := st.DeletePlan("B1"); err != nil {
		t.Fatalf("paket yang masih ada paket lain harus bisa dihapus: %v", err)
	}
	err := st.DeletePlan("A1")
	if err == nil {
		t.Fatal("paket terakhir yang dijual tidak boleh terhapus")
	}
	var inUse ErrInUse
	if !errors.As(err, &inUse) {
		t.Fatalf("galat = %v, mau ErrInUse", err)
	}
	if !strings.Contains(inUse.Msg, "satu paket") {
		t.Errorf("pesan %q harus menjelaskan bahwa minimal harus ada satu paket", inUse.Msg)
	}
	if _, err := st.PlanByCode("A1"); err != nil {
		t.Errorf("paket A1 ikut hilang: %v", err)
	}
}

func TestPlanUsageCountsUnpaidCustomerUnderFallback(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// Pelanggan baru yang belum pernah membayar: masa berlakunya diberikan
	// lewat tanggal, jadi tidak ada pembayaran yang disetujui.
	var instID string
	if err := st.db.QueryRow(`SELECT id FROM instances LIMIT 1`).Scan(&instID); err != nil {
		t.Fatalf("baca instance: %v", err)
	}
	baru, err := st.CreateCustomer(NewCustomer{
		Name:        "Baru",
		Institution: "Baru.net",
		SessionName: "baru",
		InstanceID:  instID,
		ValidUntil:  today().AddDate(0, 0, 30).Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}

	usage, err := st.PlanUsage()
	if err != nil {
		t.Fatalf("PlanUsage: %v", err)
	}
	// Dua pelanggan contoh punya P1M yang disetujui; yang baru belum pernah
	// membayar dan harus dihitung ke paket cadangan, P1M juga.
	if usage["P1M"] != 3 {
		t.Errorf("PlanUsage[P1M] = %d, mau 3 (termasuk pelanggan tanpa pembayaran)", usage["P1M"])
	}
	if n, ada := usage[""]; ada || n != 0 {
		t.Errorf("PlanUsage[\"\"] = %d (ada = %v): pelanggan tanpa pembayaran tidak boleh punya kunci sendiri", n, ada)
	}

	// Paket cadangan harus sama dengan yang dipakai halaman admin.
	srv := NewServer(Config{BaseURL: "http://portal.test"}, st)
	if got := srv.lastPlanCode(baru.ID); got != "P1M" {
		t.Errorf("lastPlanCode pelanggan tanpa pembayaran = %q, mau P1M", got)
	}
}

func TestUniqueViolationDetection(t *testing.T) {
	// modernc melaporkan bentrok PRIMARY KEY sebagai teks galat seperti ini:
	// "constraint failed: UNIQUE constraint failed: plans.code (1555)".
	if !isUniqueViolation(errors.New("constraint failed: UNIQUE constraint failed: plans.code (1555)")) {
		t.Error("pelanggaran UNIQUE tidak terdeteksi")
	}
	if isUniqueViolation(nil) {
		t.Error("galat kosong dianggap pelanggaran UNIQUE")
	}
	if isUniqueViolation(errors.New("disk I/O error")) {
		t.Error("galat yang bukan pelanggaran UNIQUE ikut terdeteksi")
	}
}

// TestApproveClaimLegacyWithoutMonths menjaga baris klaim dari basis data
// sebelum migrasi v2: durasinya belum tersimpan (0), jadi penyetujuan masih
// memakai durasi paket seperti semula.
func TestApproveClaimLegacyWithoutMonths(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	cust := customerFromSeed(t, st, "taufiq")

	// Klaim lama dibuat lewat basis data, bukan CreateClaim, supaya kolom
	// months-nya tetap 0 seperti baris sebelum migrasi.
	if _, err := st.db.Exec(
		`INSERT INTO payments (id, customer_id, plan_code, amount, months, status, ref, created_at)
		 VALUES ('p-lama', ?, 'P3M', 135000, 0, 'pending', 'NOC-LAMA', ?)`,
		cust.ID, Now().Format(time.RFC3339)); err != nil {
		t.Fatalf("insert klaim lama: %v", err)
	}

	until, err := st.ApproveClaim("p-lama")
	if err != nil {
		t.Fatalf("ApproveClaim klaim lama: %v", err)
	}
	want := addMonths(mustDate(t, cust.ValidUntil), 3)
	if !until.Equal(want) {
		t.Fatalf("berlaku sampai %s, mau %s (durasi diambil dari paket P3M)",
			until.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// TestCreateClaimUnknownPlan menjaga jawaban INSERT atomik: paket yang tidak
// ada tetap menghasilkan ErrNotFound, bukan galat basis data mentah.
func TestCreateClaimUnknownPlan(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	cust := customerFromSeed(t, st, "taufiq")

	if _, err := st.CreateClaim(cust.ID, "TIDAK-ADA"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("galat = %v, mau ErrNotFound", err)
	}
	if n := countRows(t, st, `SELECT COUNT(*) FROM payments WHERE plan_code = 'TIDAK-ADA'`); n != 0 {
		t.Errorf("klaim untuk paket yang tidak ada ikut tersimpan: %d baris", n)
	}
}
