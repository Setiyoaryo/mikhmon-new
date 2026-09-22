// Package portal berisi backend portal langganan NOCIFY Billing.
//
// Portal ini adalah satu-satunya tempat yang memegang harga dan QRIS, dan
// satu-satunya tempat yang menentukan sebuah panel Mikhmon masih aktif atau
// tidak. Panel pelanggan hanya bertanya lewat /api/v1/heartbeat.
package portal

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Location dipakai untuk semua perhitungan tanggal langganan. Semua pelanggan
// berada di zona yang sama, jadi disamakan supaya "sisa hari" tidak berbeda
// antara portal dan panel.
var loc = jakarta()

func jakarta() *time.Location {
	l, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// tzdata belum ada: tetap jalan dengan offset tetap.
		return time.FixedZone("WIB", 7*3600)
	}
	return l
}

// Now mengembalikan waktu sekarang di zona yang dipakai portal.
func Now() time.Time { return time.Now().In(loc) }

// today adalah tengah malam hari ini, dasar semua hitungan "sisa hari".
func today() time.Time {
	n := Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}

var (
	// ErrNotFound dikembalikan kalau barisnya tidak ada.
	ErrNotFound = errors.New("not found")
	// ErrPending dikembalikan kalau pelanggan sudah punya klaim yang menunggu.
	ErrPending = errors.New("claim already pending")
)

// Store membungkus basis data portal.
type Store struct{ db *sql.DB }

// OpenStore membuka (dan kalau perlu membuat) basis data SQLite.
func OpenStore(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite menulis satu per satu; satu koneksi menghindari SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Close menutup basis data.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS customers (
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

CREATE TABLE IF NOT EXISTS instances (
  id          TEXT PRIMARY KEY,
  token       TEXT NOT NULL,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  router_name TEXT NOT NULL DEFAULT '',
  version     TEXT NOT NULL DEFAULT '',
  last_seen   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS plans (
  code   TEXT PRIMARY KEY,
  label  TEXT NOT NULL,
  months INTEGER NOT NULL,
  price  INTEGER NOT NULL,
  note   TEXT NOT NULL DEFAULT '',
  sort   INTEGER NOT NULL DEFAULT 0,
  sale   INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS payments (
  id          TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  plan_code   TEXT NOT NULL,
  amount      INTEGER NOT NULL,
  status      TEXT NOT NULL,
  ref         TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  decided_at  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS events (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  at   TEXT NOT NULL,
  text TEXT NOT NULL
);
`)
	return err
}

/* ------------------------------------------------------------------ model */

// Plan adalah satu paket sewa.
type Plan struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Months int    `json:"months"`
	Price  int64  `json:"price"`
	Note   string `json:"note"`
}

// Customer adalah satu pelanggan beserta status langganannya.
type Customer struct {
	ID          string
	Name        string
	Institution string
	WA          string
	PayToken    string
	ValidFrom   string
	ValidUntil  string
	Suspended   bool
	CreatedAt   string
	PlanCode    string // paket terakhir yang dibayar
}

// Instance adalah satu pemasangan panel Mikhmon milik pelanggan.
type Instance struct {
	ID         string
	Token      string
	CustomerID string
	RouterName string
	Version    string
	LastSeen   string
}

// Claim adalah klaim pembayaran dari pelanggan.
type Claim struct {
	ID          string
	CustomerID  string
	Customer    string
	Institution string
	PlanCode    string
	PlanLabel   string
	Amount      int64
	Status      string
	Ref         string
	CreatedAt   string
}

/* ------------------------------------------------------------------ plans */

// Plans mengembalikan paket yang dijual, terurut.
func (s *Store) Plans(saleOnly bool) ([]Plan, error) {
	q := `SELECT code, label, months, price, note FROM plans`
	if saleOnly {
		q += ` WHERE sale = 1`
	}
	q += ` ORDER BY sort, price`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.Code, &p.Label, &p.Months, &p.Price, &p.Note); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PlanByCode mengambil satu paket.
func (s *Store) PlanByCode(code string) (Plan, error) {
	var p Plan
	err := s.db.QueryRow(
		`SELECT code, label, months, price, note FROM plans WHERE code = ?`, code,
	).Scan(&p.Code, &p.Label, &p.Months, &p.Price, &p.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

/* -------------------------------------------------------------- customers */

func (s *Store) scanCustomer(row interface{ Scan(...any) error }) (Customer, error) {
	var c Customer
	var sus int
	err := row.Scan(&c.ID, &c.Name, &c.Institution, &c.WA, &c.PayToken,
		&c.ValidFrom, &c.ValidUntil, &sus, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	c.Suspended = sus != 0
	return c, err
}

const customerCols = `id, name, institution, wa, pay_token, valid_from, valid_until, suspended, created_at`

// CustomerByPayToken mencari pelanggan dari token halaman pembayarannya.
func (s *Store) CustomerByPayToken(token string) (Customer, error) {
	return s.scanCustomer(s.db.QueryRow(
		`SELECT `+customerCols+` FROM customers WHERE pay_token = ?`, token))
}

// CustomerByID mencari pelanggan dari id-nya.
func (s *Store) CustomerByID(id string) (Customer, error) {
	return s.scanCustomer(s.db.QueryRow(
		`SELECT `+customerCols+` FROM customers WHERE id = ?`, id))
}

// AllCustomers mengembalikan semua pelanggan, terurut dari yang paling mendesak.
func (s *Store) AllCustomers() ([]Customer, error) {
	rows, err := s.db.Query(`SELECT ` + customerCols + ` FROM customers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Customer
	for rows.Next() {
		c, err := s.scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetSuspended menangguhkan atau mengaktifkan kembali pelanggan.
func (s *Store) SetSuspended(id string, suspended bool) error {
	v := 0
	if suspended {
		v = 1
	}
	res, err := s.db.Exec(`UPDATE customers SET suspended = ? WHERE id = ?`, v, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

/* -------------------------------------------------------------- instances */

// InstanceByID mengambil instance beserta token-nya (untuk heartbeat).
func (s *Store) InstanceByID(id string) (Instance, error) {
	var i Instance
	err := s.db.QueryRow(
		`SELECT id, token, customer_id, router_name, version, last_seen FROM instances WHERE id = ?`, id,
	).Scan(&i.ID, &i.Token, &i.CustomerID, &i.RouterName, &i.Version, &i.LastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return i, ErrNotFound
	}
	return i, err
}

// FirstInstance mengambil instance milik pelanggan (satu pelanggan satu instance).
func (s *Store) FirstInstance(customerID string) (Instance, error) {
	var i Instance
	err := s.db.QueryRow(
		`SELECT id, token, customer_id, router_name, version, last_seen FROM instances
		 WHERE customer_id = ? ORDER BY rowid LIMIT 1`, customerID,
	).Scan(&i.ID, &i.Token, &i.CustomerID, &i.RouterName, &i.Version, &i.LastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return i, ErrNotFound
	}
	return i, err
}

// TouchInstance mencatat kapan terakhir panel melapor.
func (s *Store) TouchInstance(id, version string) error {
	_, err := s.db.Exec(
		`UPDATE instances SET last_seen = ?, version = CASE WHEN ? <> '' THEN ? ELSE version END WHERE id = ?`,
		Now().Format(time.RFC3339), version, version, id)
	return err
}

/* --------------------------------------------------------------- payments */

const claimCols = `p.id, p.customer_id, c.name, c.institution, p.plan_code, p.amount, p.status, p.ref, p.created_at`

func scanClaim(row interface{ Scan(...any) error }) (Claim, error) {
	var c Claim
	err := row.Scan(&c.ID, &c.CustomerID, &c.Customer, &c.Institution,
		&c.PlanCode, &c.Amount, &c.Status, &c.Ref, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// PendingClaim mengembalikan klaim yang belum diputuskan, kalau ada.
func (s *Store) PendingClaim(customerID string) (Claim, error) {
	return scanClaim(s.db.QueryRow(
		`SELECT `+claimCols+` FROM payments p JOIN customers c ON c.id = p.customer_id
		 WHERE p.customer_id = ? AND p.status = 'pending' ORDER BY p.created_at DESC LIMIT 1`, customerID))
}

// PendingClaims mengembalikan semua klaim yang menunggu keputusan.
func (s *Store) PendingClaims() ([]Claim, error) {
	rows, err := s.db.Query(
		`SELECT ` + claimCols + ` FROM payments p JOIN customers c ON c.id = p.customer_id
		 WHERE p.status = 'pending' ORDER BY p.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Claim
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateClaim membuat klaim pembayaran baru. Kalau sudah ada yang menunggu,
// klaim yang itu yang dikembalikan dengan ErrPending.
func (s *Store) CreateClaim(customerID, planCode string) (Claim, error) {
	if existing, err := s.PendingClaim(customerID); err == nil {
		return existing, ErrPending
	} else if !errors.Is(err, ErrNotFound) {
		return Claim{}, err
	}

	plan, err := s.PlanByCode(planCode)
	if err != nil {
		return Claim{}, err
	}

	id := "p" + randHex(6)
	ref := "NOC-" + strings.ToUpper(randHex(2)) + "-" + strings.ToUpper(randHex(2))
	now := Now().Format(time.RFC3339)

	_, err = s.db.Exec(
		`INSERT INTO payments (id, customer_id, plan_code, amount, status, ref, created_at)
		 VALUES (?, ?, ?, ?, 'pending', ?, ?)`,
		id, customerID, plan.Code, plan.Price, ref, now)
	if err != nil {
		return Claim{}, err
	}
	return s.ClaimByID(id)
}

// ClaimByID mengambil satu klaim.
func (s *Store) ClaimByID(id string) (Claim, error) {
	return scanClaim(s.db.QueryRow(
		`SELECT `+claimCols+` FROM payments p JOIN customers c ON c.id = p.customer_id WHERE p.id = ?`, id))
}

// ApproveClaim menandai klaim disetujui dan memperpanjang langganan.
// Kalau langganannya masih berjalan, ditambahkan dari tanggal berakhirnya,
// jadi pelanggan tidak kehilangan sisa waktunya.
func (s *Store) ApproveClaim(id string) (time.Time, error) {
	claim, err := s.ClaimByID(id)
	if err != nil {
		return time.Time{}, err
	}
	if claim.Status != "pending" {
		return time.Time{}, fmt.Errorf("claim %s sudah %s", id, claim.Status)
	}
	plan, err := s.PlanByCode(claim.PlanCode)
	if err != nil {
		return time.Time{}, err
	}
	cust, err := s.CustomerByID(claim.CustomerID)
	if err != nil {
		return time.Time{}, err
	}

	from := today()
	start := c0(cust.ValidUntil)
	if start != "" {
		if t, perr := time.ParseInLocation("2006-01-02", start, loc); perr == nil && t.After(from) {
			from = t
		}
	}
	// Bulan dihitung dengan menjepit ke hari terakhir bulan tujuan, sama seperti
	// alat pembuat lisensi sebelumnya: 31 Jan + 1 bulan harus 28/29 Feb.
	until := addMonths(from, plan.Months)

	tx, err := s.db.Begin()
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE payments SET status = 'approved', decided_at = ? WHERE id = ?`,
		Now().Format(time.RFC3339), id); err != nil {
		return time.Time{}, err
	}
	if _, err := tx.Exec(`UPDATE customers SET valid_from = ?, valid_until = ?, suspended = 0 WHERE id = ?`,
		from.Format("2006-01-02"), until.Format("2006-01-02"), claim.CustomerID); err != nil {
		return time.Time{}, err
	}
	if _, err := tx.Exec(`INSERT INTO events (at, text) VALUES (?, ?)`,
		Now().Format(time.RFC3339),
		fmt.Sprintf("Pembayaran %s (%s) disetujui. Berlaku sampai %s.",
			claim.Institution, plan.Label, until.Format("2006-01-02"))); err != nil {
		return time.Time{}, err
	}
	if err := tx.Commit(); err != nil {
		return time.Time{}, err
	}
	return until, nil
}

// RejectClaim menandai klaim ditolak.
func (s *Store) RejectClaim(id string) error {
	claim, err := s.ClaimByID(id)
	if err != nil {
		return err
	}
	if claim.Status != "pending" {
		return fmt.Errorf("claim %s sudah %s", id, claim.Status)
	}
	if _, err := s.db.Exec(`UPDATE payments SET status = 'rejected', decided_at = ? WHERE id = ?`,
		Now().Format(time.RFC3339), id); err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO events (at, text) VALUES (?, ?)`,
		Now().Format(time.RFC3339),
		fmt.Sprintf("Klaim %s ditolak.", claim.Institution))
	return err
}

// RevenueThisMonth menjumlahkan pembayaran yang disetujui bulan ini.
func (s *Store) RevenueThisMonth() (int64, error) {
	first := time.Date(Now().Year(), Now().Month(), 1, 0, 0, 0, 0, loc)
	var total sql.NullInt64
	err := s.db.QueryRow(
		`SELECT SUM(amount) FROM payments WHERE status = 'approved' AND decided_at >= ?`,
		first.Format(time.RFC3339)).Scan(&total)
	return total.Int64, err
}

/* ----------------------------------------------------------------- events */

// LogEvent mencatat satu baris catatan aktivitas.
func (s *Store) LogEvent(text string) error {
	_, err := s.db.Exec(`INSERT INTO events (at, text) VALUES (?, ?)`, Now().Format(time.RFC3339), text)
	return err
}

// Event adalah satu baris catatan aktivitas.
type Event struct {
	At   string
	Text string
}

// RecentEvents mengambil catatan terbaru.
func (s *Store) RecentEvents(limit int) ([]Event, error) {
	rows, err := s.db.Query(`SELECT at, text FROM events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.At, &e.Text); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

/* ------------------------------------------------------------------ seed */

// Seed mengisi paket bawaan dan, kalau basis datanya masih kosong, satu
// pelanggan contoh supaya portalnya bisa langsung dicoba.
func (s *Store) Seed() error {
	var planCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM plans`).Scan(&planCount); err != nil {
		return err
	}
	if planCount == 0 {
		defaults := []Plan{
			{Code: "P1M", Label: "1 Bulan", Months: 1, Price: 50000, Note: ""},
			{Code: "P3M", Label: "3 Bulan", Months: 3, Price: 135000, Note: "hemat 10%"},
			{Code: "P6M", Label: "6 Bulan", Months: 6, Price: 250000, Note: "hemat 17%"},
			{Code: "P12M", Label: "12 Bulan", Months: 12, Price: 450000, Note: "hemat 25%"},
		}
		for i, p := range defaults {
			if _, err := s.db.Exec(
				`INSERT INTO plans (code, label, months, price, note, sort, sale) VALUES (?, ?, ?, ?, ?, ?, 1)`,
				p.Code, p.Label, p.Months, p.Price, p.Note, i); err != nil {
				return err
			}
		}
	}

	var custCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM customers`).Scan(&custCount); err != nil {
		return err
	}
	if custCount > 0 {
		return nil
	}

	// Pelanggan contoh ini yang Anda pakai untuk mencoba sendiri: hotspot
	// taufiq.nocify.id. Tanggalnya diisi 4 hari lagi supaya keadaan "segera
	// berakhir" langsung kelihatan.
	id := "c" + randHex(6)
	custID := id
	until := today().AddDate(0, 0, 4).Format("2006-01-02")
	from := today().AddDate(0, 0, -26).Format("2006-01-02")
	if _, err := s.db.Exec(
		`INSERT INTO customers (id, name, institution, wa, pay_token, valid_from, valid_until, suspended, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		custID, "Taufiq", "Taufiq.net", "6285139495106",
		"NOC-"+strings.ToUpper(randHex(2))+"-"+strings.ToUpper(randHex(2)),
		from, until, Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`INSERT INTO instances (id, token, customer_id, router_name, version, last_seen)
		 VALUES (?, ?, ?, ?, ?, '')`,
		strings.ToUpper(randHex(6)), randHex(16), custID, "CHR-HOTSPOT", ""); err != nil {
		return err
	}
	// Pembayaran pertama yang sudah disetujui, supaya catatannya utuh: tanpa ini
	// pelanggan punya tanggal berlaku tetapi tidak punya paket yang bisa dibaca.
	paid := Now().AddDate(0, 0, -26).Format(time.RFC3339)
	if _, err := s.db.Exec(
		`INSERT INTO payments (id, customer_id, plan_code, amount, status, ref, created_at, decided_at)
		 VALUES (?, ?, 'P1M', 50000, 'approved', ?, ?, ?)`,
		"p"+randHex(6), custID,
		"NOC-"+strings.ToUpper(randHex(2))+"-"+strings.ToUpper(randHex(2)),
		paid, paid); err != nil {
		return err
	}
	return s.LogEvent("Portal disiapkan. Pelanggan contoh dibuat untuk uji coba.")
}

/* ------------------------------------------------------------------ util */

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(b)
}

// c0 mengembalikan s kalau ada isinya, atau "" kalau kosong.
func c0(s string) string { return strings.TrimSpace(s) }

// addMonths menambah bulan dengan menjepit ke hari terakhir bulan tujuan.
func addMonths(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	total := int(m) + months
	y += (total - 1) / 12
	m = time.Month((total-1)%12 + 1)
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, loc).Day()
	if d > last {
		d = last
	}
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}
