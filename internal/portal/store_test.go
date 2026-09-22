package portal

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

/* ------------------------------------------------------------------ bantu */

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenStore(filepath.Join(t.TempDir(), "portal.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func userVersion(t *testing.T, st *Store) int {
	t.Helper()
	var v int
	if err := st.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("PRAGMA user_version: %v", err)
	}
	return v
}

func hasColumn(t *testing.T, st *Store, table, column string) bool {
	t.Helper()
	rows, err := st.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s): %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, typ        string
			dflt             any
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows: %v", err)
	}
	return false
}

func countRows(t *testing.T, st *Store, query string) int {
	t.Helper()
	var n int
	if err := st.db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func doHeartbeat(t *testing.T, h http.Handler, body map[string]any) (int, map[string]any) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("balasan bukan JSON: %q", rec.Body.String())
		}
	}
	return rec.Code, out
}

/* --------------------------------------------------------------- migrasi */

func TestMigrateFreshDatabase(t *testing.T) {
	st := openTestStore(t)

	if v := userVersion(t, st); v != schemaVersion {
		t.Fatalf("user_version = %d, mau %d", v, schemaVersion)
	}
	for _, col := range []string{"instance_id", "session_name"} {
		if !hasColumn(t, st, "customers", col) {
			t.Errorf("customers.%s tidak ada di basis data baru", col)
		}
	}
	for _, col := range []string{"name", "kind", "router_name"} {
		if !hasColumn(t, st, "instances", col) {
			t.Errorf("instances.%s tidak ada di basis data baru", col)
		}
	}
	if hasColumn(t, st, "instances", "customer_id") {
		t.Error("instances.customer_id masih ada di basis data baru")
	}
	// DDL basis data baru harus langsung memuat durasi pembelian yang
	// dibekukan, bukan menunggu migrasi menambahkannya.
	if !hasColumn(t, st, "payments", "months") {
		t.Error("payments.months tidak ada di basis data baru")
	}
}

// legacyDDL adalah bentuk basis data sebelum migrasi (satu instance satu
// pelanggan), disalin dari versi lama supaya jalur migrasinya benar-benar diuji.
const legacyDDL = `
CREATE TABLE customers (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  institution TEXT NOT NULL,
  wa          TEXT NOT NULL DEFAULT '',
  pay_token   TEXT NOT NULL UNIQUE,
  valid_from  TEXT NOT NULL DEFAULT '',
  valid_until TEXT NOT NULL DEFAULT '',
  suspended   INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL
);
CREATE TABLE instances (
  id          TEXT PRIMARY KEY,
  token       TEXT NOT NULL,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  router_name TEXT NOT NULL DEFAULT '',
  version     TEXT NOT NULL DEFAULT '',
  last_seen   TEXT NOT NULL DEFAULT ''
);
CREATE TABLE plans (
  code   TEXT PRIMARY KEY,
  label  TEXT NOT NULL,
  months INTEGER NOT NULL,
  price  INTEGER NOT NULL,
  note   TEXT NOT NULL DEFAULT '',
  sort   INTEGER NOT NULL DEFAULT 0,
  sale   INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE payments (
  id          TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  plan_code   TEXT NOT NULL,
  amount      INTEGER NOT NULL,
  status      TEXT NOT NULL,
  ref         TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  decided_at  TEXT NOT NULL DEFAULT ''
);
CREATE TABLE events (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  at   TEXT NOT NULL,
  text TEXT NOT NULL
);`

// writeLegacyDatabase membuat basis data berisi satu pelanggan dengan
// institution dan satu pelanggan tanpa institution, masing-masing punya
// instance sendiri, plus pembayaran yang harus selamat dari migrasi.
func writeLegacyDatabase(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("buka basis data lama: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(legacyDDL); err != nil {
		t.Fatalf("buat skema lama: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO customers (id, name, institution, wa, pay_token, valid_from, valid_until, suspended, created_at)
		VALUES ('c1', 'Taufiq', 'Taufiq.net', '6285139495106', 'NOC-FF1D-D59D', '2026-08-27', '2026-12-26', 0, '2026-09-22T09:00:58+07:00'),
		       ('c2', 'Hendra', '', '', 'NOC-ABCD-1234', '', '', 0, '2026-09-22T09:00:58+07:00');
		INSERT INTO instances (id, token, customer_id, router_name, version, last_seen)
		VALUES ('C3492E0769AF', '1871609b0ce354b875e9491dbd3ad219', 'c1', 'CHR-HOTSPOT', '3.20', '2026-09-22T09:23:22+07:00'),
		       ('OLD2', 'tok-kedua', 'c2', 'CHR-LAMA', '3.19', '');
		INSERT INTO plans (code, label, months, price, note, sort, sale)
		VALUES ('P1M', '1 Bulan', 1, 50000, '', 0, 1);
		INSERT INTO payments (id, customer_id, plan_code, amount, status, ref, created_at, decided_at)
		VALUES ('p1', 'c1', 'P1M', 50000, 'approved', 'NOC-3EA3-0366', '2026-08-27T09:00:58+07:00', '2026-08-27T09:00:58+07:00'),
		       ('p2', 'c1', 'P3M', 135000, 'approved', 'NOC-5192-EE6C', '2026-09-22T09:01:11+07:00', '2026-09-22T09:01:44+07:00'),
		       ('p3', 'c2', 'P1M', 50000, 'pending', 'NOC-1111-2222', '2026-09-22T09:02:00+07:00', '');
		INSERT INTO events (at, text) VALUES ('2026-09-22T09:00:58+07:00', 'catatan lama');`); err != nil {
		t.Fatalf("isi basis data lama: %v", err)
	}
}

func TestMigrateLegacyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "portal.db")
	writeLegacyDatabase(t, path)

	st, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore atas basis data lama: %v", err)
	}
	defer st.Close()

	if v := userVersion(t, st); v != schemaVersion {
		t.Fatalf("user_version = %d, mau %d", v, schemaVersion)
	}
	if hasColumn(t, st, "instances", "customer_id") {
		t.Error("instances.customer_id belum hilang setelah migrasi")
	}

	// Instance lama jadi deployment dedicated dengan id dan isi yang sama.
	inst, err := st.InstanceByID("C3492E0769AF")
	if err != nil {
		t.Fatalf("InstanceByID setelah migrasi: %v", err)
	}
	if inst.Kind != "dedicated" {
		t.Errorf("kind = %q, mau \"dedicated\"", inst.Kind)
	}
	if inst.Token != "1871609b0ce354b875e9491dbd3ad219" || inst.RouterName != "CHR-HOTSPOT" ||
		inst.Version != "3.20" || inst.LastSeen != "2026-09-22T09:23:22+07:00" {
		t.Errorf("data instance lama berbubah: %+v", inst)
	}
	if inst.Name != "Taufiq.net" {
		t.Errorf("name = %q, mau \"Taufiq.net\" (institution pelanggan)", inst.Name)
	}

	cust, err := st.CustomerByID("c1")
	if err != nil {
		t.Fatalf("CustomerByID(c1): %v", err)
	}
	if cust.PayToken != "NOC-FF1D-D59D" || cust.ValidUntil != "2026-12-26" {
		t.Errorf("data pelanggan lama berubah: %+v", cust)
	}
	if cust.InstanceID != "C3492E0769AF" || cust.SessionName != "taufiq" {
		t.Errorf("instance_id/session_name = %q/%q, mau \"C3492E0769AF\"/\"taufiq\"",
			cust.InstanceID, cust.SessionName)
	}

	// Tanpa institution, session_name diturunkan dari pay_token.
	cust2, err := st.CustomerByID("c2")
	if err != nil {
		t.Fatalf("CustomerByID(c2): %v", err)
	}
	if cust2.InstanceID != "OLD2" || cust2.SessionName != "noc-abcd-1234" {
		t.Errorf("instance_id/session_name c2 = %q/%q, mau \"OLD2\"/\"noc-abcd-1234\"",
			cust2.InstanceID, cust2.SessionName)
	}

	// Pembayaran, paket, dan catatan tidak boleh tersentuh.
	if n := countRows(t, st, `SELECT COUNT(*) FROM payments`); n != 3 {
		t.Errorf("jumlah payments = %d, mau 3", n)
	}
	if n := countRows(t, st, `SELECT COUNT(*) FROM plans`); n != 1 {
		t.Errorf("jumlah plans = %d, mau 1", n)
	}
	if n := countRows(t, st, `SELECT COUNT(*) FROM events`); n != 1 {
		t.Errorf("jumlah events = %d, mau 1", n)
	}
	var status string
	if err := st.db.QueryRow(`SELECT status FROM payments WHERE id = 'p3'`).Scan(&status); err != nil || status != "pending" {
		t.Errorf("payments p3 = %q/%v, mau pending", status, err)
	}
	// Langkah versi 2 menambah kolom durasi; pembayaran lama belum punya
	// durasi tersimpan, jadi nilainya 0 dan penyetujuannya memakai durasi paket.
	if !hasColumn(t, st, "payments", "months") {
		t.Error("payments.months tidak bertambah saat migrasi")
	}
	var months int
	if err := st.db.QueryRow(`SELECT months FROM payments WHERE id = 'p1'`).Scan(&months); err != nil {
		t.Fatalf("baca payments.months: %v", err)
	}
	if months != 0 {
		t.Errorf("payments.months baris lama = %d, mau 0", months)
	}

	// Migrasi kedua kali harus aman: buka ulang dari berkas yang sama.
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	st2, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore kedua kali: %v", err)
	}
	defer st2.Close()
	if n := countRows(t, st2, `SELECT COUNT(*) FROM instances`); n != 2 {
		t.Errorf("jumlah instances setelah buka ulang = %d, mau 2", n)
	}
	if n := countRows(t, st2, `SELECT COUNT(*) FROM payments`); n != 3 {
		t.Errorf("jumlah payments setelah buka ulang = %d, mau 3", n)
	}
	if c, err := st2.CustomerBySession("C3492E0769AF", "taufiq"); err != nil || c.ID != "c1" {
		t.Errorf("CustomerBySession setelah buka ulang = %+v/%v", c, err)
	}
}

/* ------------------------------------------------------------------- seed */

func TestSeedSharedPanelWithTwoCustomers(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	if n := countRows(t, st, `SELECT COUNT(*) FROM instances`); n != 1 {
		t.Fatalf("jumlah instances = %d, mau 1", n)
	}
	var id, kind string
	if err := st.db.QueryRow(`SELECT id, kind FROM instances`).Scan(&id, &kind); err != nil {
		t.Fatalf("baca instance: %v", err)
	}
	if kind != "shared" {
		t.Errorf("kind = %q, mau \"shared\"", kind)
	}

	hosted, err := st.CustomersByInstance(id)
	if err != nil {
		t.Fatalf("CustomersByInstance: %v", err)
	}
	if len(hosted) != 2 {
		t.Fatalf("jumlah pelanggan di panel bersama = %d, mau 2", len(hosted))
	}
	if hosted[0].SessionName != "hendra" || hosted[1].SessionName != "taufiq" {
		t.Errorf("session_name = %q/%q, mau hendra/taufiq", hosted[0].SessionName, hosted[1].SessionName)
	}

	// Seed lagi tidak boleh menambah contoh kedua.
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed kedua kali: %v", err)
	}
	if n := countRows(t, st, `SELECT COUNT(*) FROM customers`); n != 2 {
		t.Errorf("jumlah customers setelah Seed kedua = %d, mau 2", n)
	}
}

/* -------------------------------------------------------------- heartbeat */

func seedInstance(t *testing.T, st *Store) Instance {
	t.Helper()
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	var id string
	if err := st.db.QueryRow(`SELECT id FROM instances`).Scan(&id); err != nil {
		t.Fatalf("baca id instance: %v", err)
	}
	inst, err := st.InstanceByID(id)
	if err != nil {
		t.Fatalf("InstanceByID: %v", err)
	}
	return inst
}

func TestHeartbeatTenantResolution(t *testing.T) {
	st := openTestStore(t)
	inst := seedInstance(t, st)
	h := NewServer(Config{BaseURL: "http://portal.test"}, st).Handler()

	t.Run("tanpa tenant di panel bersama", func(t *testing.T) {
		code, body := doHeartbeat(t, h, map[string]any{
			"instance_id": inst.ID, "token": inst.Token, "version": "9.9",
		})
		if code != http.StatusBadRequest {
			t.Fatalf("status = %d, mau 400", code)
		}
		if body["error"] != "tenant_required" {
			t.Fatalf("error = %v, mau tenant_required", body["error"])
		}
	})

	t.Run("pelanggan sehat", func(t *testing.T) {
		code, body := doHeartbeat(t, h, map[string]any{
			"instance_id": inst.ID, "token": inst.Token, "version": "9.9", "tenant": "taufiq",
		})
		if code != http.StatusOK {
			t.Fatalf("status = %d, mau 200 (%v)", code, body)
		}
		if body["state"] != "active" {
			t.Errorf("state = %v, mau active", body["state"])
		}
		if body["plan"] != "1 Bulan" {
			t.Errorf("plan = %v, mau 1 Bulan", body["plan"])
		}
		if body["days_left"].(float64) != 26 {
			t.Errorf("days_left = %v, mau 26", body["days_left"])
		}
		if body["pay_url"] == "" || body["server_time"] == "" || body["expires_at"] == "" || body["message"] == "" {
			t.Errorf("ada field balasan yang kosong: %v", body)
		}
	})

	t.Run("pelanggan berakhir", func(t *testing.T) {
		code, body := doHeartbeat(t, h, map[string]any{
			"instance_id": inst.ID, "token": inst.Token, "tenant": "hendra",
		})
		if code != http.StatusOK {
			t.Fatalf("status = %d, mau 200 (%v)", code, body)
		}
		if body["state"] != "expired" {
			t.Errorf("state = %v, mau expired", body["state"])
		}
	})

	t.Run("tenant tidak dikenal", func(t *testing.T) {
		code, body := doHeartbeat(t, h, map[string]any{
			"instance_id": inst.ID, "token": inst.Token, "tenant": "tidak-ada",
		})
		if code != http.StatusNotFound || body["error"] != "unknown_tenant" {
			t.Fatalf("status/error = %d/%v, mau 404/unknown_tenant", code, body["error"])
		}
	})

	t.Run("token salah", func(t *testing.T) {
		code, body := doHeartbeat(t, h, map[string]any{
			"instance_id": inst.ID, "token": "salah", "tenant": "taufiq",
		})
		if code != http.StatusUnauthorized || body["error"] != "bad_token" {
			t.Fatalf("status/error = %d/%v, mau 401/bad_token", code, body["error"])
		}
	})

	t.Run("instance tidak dikenal", func(t *testing.T) {
		code, body := doHeartbeat(t, h, map[string]any{
			"instance_id": "tidak-ada", "token": inst.Token, "tenant": "taufiq",
		})
		if code != http.StatusNotFound || body["error"] != "unknown_instance" {
			t.Fatalf("status/error = %d/%v, mau 404/unknown_instance", code, body["error"])
		}
	})
}

func TestHeartbeatDedicatedWithoutTenant(t *testing.T) {
	st := openTestStore(t)
	now := Now().Format("2006-01-02")
	if _, err := st.db.Exec(`INSERT INTO instances (id, token, name, kind) VALUES ('I1', 'tok1', 'Panel Khusus', 'dedicated')`); err != nil {
		t.Fatalf("insert instance: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO customers (id, name, institution, wa, pay_token, valid_from, valid_until, suspended, created_at, instance_id, session_name)
		VALUES ('c1', 'Solo', 'Solo.net', '', 'NOC-0000-0001', ?, ?, 0, ?, 'I1', 'solo')`,
		now, now, Now().Format("2006-01-02T15:04:05Z07:00")); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	h := NewServer(Config{BaseURL: "http://portal.test"}, st).Handler()

	code, body := doHeartbeat(t, h, map[string]any{"instance_id": "I1", "token": "tok1"})
	if code != http.StatusOK {
		t.Fatalf("status = %d, mau 200 (%v)", code, body)
	}
	if body["state"] != "warning" {
		t.Errorf("state = %v, mau warning", body["state"])
	}

	// Deployment tanpa pelanggan sama sekali tetap 404 unknown_instance.
	if _, err := st.db.Exec(`INSERT INTO instances (id, token, name, kind) VALUES ('I2', 'tok2', 'Kosong', 'shared')`); err != nil {
		t.Fatalf("insert instance kosong: %v", err)
	}
	code, body = doHeartbeat(t, h, map[string]any{"instance_id": "I2", "token": "tok2"})
	if code != http.StatusNotFound || body["error"] != "unknown_instance" {
		t.Fatalf("status/error = %d/%v, mau 404/unknown_instance", code, body["error"])
	}
}

/* -------------------------------------------------------- migrasi v1 ke v2 */

// writeV1Database membuat basis data yang sudah ditandai versi 1 tetapi tabel
// payments-nya belum punya kolom months, seperti basis data yang dibuat
// sebelum langkah versi 2 ada. Satu baris pembayaran disimpan supaya kenaikan
// versi bisa diperiksa tidak menghilangkan data.
func writeV1Database(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("buka basis data versi 1: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(legacyDDL); err != nil {
		t.Fatalf("buat skema versi 1: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO customers (id, name, institution, wa, pay_token, valid_from, valid_until, suspended, created_at)
		VALUES ('c1', 'Taufiq', 'Taufiq.net', '', 'NOC-FF1D-D59D', '2026-08-27', '2026-12-26', 0, '2026-09-22T09:00:58+07:00');
		INSERT INTO payments (id, customer_id, plan_code, amount, status, ref, created_at, decided_at)
		VALUES ('p1', 'c1', 'P1M', 50000, 'approved', 'NOC-3EA3-0366', '2026-08-27T09:00:58+07:00', '2026-08-27T09:00:58+07:00');`); err != nil {
		t.Fatalf("isi basis data versi 1: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatalf("tandai versi 1: %v", err)
	}
}

// TestMigrateV1ToV2KeepsRows memastikan basis data versi 1 naik ke versi 2
// tanpa kehilangan baris: kolom months bertambah dan barisnya masih utuh.
func TestMigrateV1ToV2KeepsRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "portal.db")
	writeV1Database(t, path)

	st, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore atas basis data versi 1: %v", err)
	}
	defer st.Close()

	if v := userVersion(t, st); v != 2 {
		t.Fatalf("user_version = %d, mau 2", v)
	}
	if !hasColumn(t, st, "payments", "months") {
		t.Fatal("kolom payments.months tidak bertambah saat naik dari versi 1")
	}
	if n := countRows(t, st, `SELECT COUNT(*) FROM payments`); n != 1 {
		t.Fatalf("jumlah payments = %d, mau 1 (baris lama hilang)", n)
	}
	var plan string
	var amount, months int
	if err := st.db.QueryRow(`SELECT plan_code, amount, months FROM payments WHERE id = 'p1'`).
		Scan(&plan, &amount, &months); err != nil {
		t.Fatalf("baca pembayaran lama: %v", err)
	}
	if plan != "P1M" || amount != 50000 || months != 0 {
		t.Errorf("baris lama berubah: plan=%q amount=%d months=%d, mau P1M/50000/0", plan, amount, months)
	}
}
