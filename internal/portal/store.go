// Package portal berisi backend portal langganan NOCIFY Billing.
//
// Portal ini adalah satu-satunya tempat yang memegang harga dan QRIS, dan
// satu-satunya tempat yang menentukan sebuah panel Mikhmon masih aktif atau
// tidak. Panel pelanggan hanya bertanya lewat /api/v1/heartbeat.
package portal

import (
	"context"
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

// schemaVersion adalah versi skema yang disimpan di PRAGMA user_version.
// Angka ini dinaikkan setiap kali bentuk tabel berubah, supaya basis data lama
// bisa ditingkatkan di tempat tanpa kehilangan baris.
const schemaVersion = 2

func (s *Store) migrate() error {
	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	// Langkah dipilih dari versi yang tersimpan, satu per satu: basis data
	// yang tertinggal dua langkah harus menjalankan keduanya, bukan melompat
	// ke langkah terakhir.
	if v < 1 {
		if err := s.migrateToV1(); err != nil {
			return err
		}
	}
	if v < 2 {
		return s.migrateToV2()
	}
	return nil
}

// migrateToV1 mengubah model lama menjadi model pemasangan bersama.
//
// Seluruh perubahan, termasuk penanda user_version, dijalankan dalam satu
// transaksi. Kalau prosesnya mati di tengah, basis datanya kembali ke keadaan
// semula, dan langkah yang belum selesai akan dicoba lagi saat dibuka
// berikutnya.
func (s *Store) migrateToV1() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	hasInstances, err := tableExists(tx, "instances")
	if err != nil {
		return err
	}
	legacy := false
	if hasInstances {
		// Kolom instances.customer_id hanya ada di model lama.
		if legacy, err = columnExists(tx, "instances", "customer_id"); err != nil {
			return err
		}
	}

	for _, ddl := range []string{customersDDL, plansDDL, paymentsDDL, eventsDDL} {
		if _, err := tx.Exec(ddl); err != nil {
			return err
		}
	}
	// Tabel customers yang sudah ada tidak diubah oleh CREATE TABLE IF NOT
	// EXISTS, jadi dua kolom baru ditambahkan sendiri.
	if err := addColumn(tx, "customers", "instance_id", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := addColumn(tx, "customers", "session_name", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}

	switch {
	case legacy:
		if err := migrateLegacyInstances(tx); err != nil {
			return err
		}
	case !hasInstances:
		if _, err := tx.Exec(instancesDDL("instances")); err != nil {
			return err
		}
	}

	// Ditulis 1, bukan schemaVersion: setelah langkah ini masih ada langkah
	// versi 2 yang harus dijalankan, jadi penandanya tidak boleh melompat ke
	// versi terakhir.
	if _, err := tx.Exec(`PRAGMA user_version = 1`); err != nil {
		return err
	}
	return tx.Commit()
}

// migrateToV2 membekukan durasi pembelian di klaim: kolom payments.months
// mencatat berapa bulan yang dibeli saat klaim dibuat, jadi mengubah paket
// sesudahnya tidak mengubah masa berlaku yang didapat pelanggan.
//
// Sama seperti v1, perubahan dan penanda versinya dijalankan dalam satu
// transaksi; menjalankannya lagi aman karena addColumn melewati kolom yang
// sudah ada.
func (s *Store) migrateToV2() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := addColumn(tx, "payments", "months", `INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`PRAGMA user_version = 2`); err != nil {
		return err
	}
	return tx.Commit()
}

const customersDDL = `
CREATE TABLE IF NOT EXISTS customers (
  id           TEXT PRIMARY KEY,
  name         TEXT NOT NULL,
  institution  TEXT NOT NULL,
  wa           TEXT NOT NULL DEFAULT '',
  pay_token    TEXT NOT NULL UNIQUE,
  valid_from   TEXT NOT NULL DEFAULT '',
  valid_until  TEXT NOT NULL DEFAULT '',
  suspended    INTEGER NOT NULL DEFAULT 0,
  created_at   TEXT NOT NULL,
  instance_id  TEXT NOT NULL DEFAULT '',
  session_name TEXT NOT NULL DEFAULT ''
);`

const plansDDL = `
CREATE TABLE IF NOT EXISTS plans (
  code   TEXT PRIMARY KEY,
  label  TEXT NOT NULL,
  months INTEGER NOT NULL,
  price  INTEGER NOT NULL,
  note   TEXT NOT NULL DEFAULT '',
  sort   INTEGER NOT NULL DEFAULT 0,
  sale   INTEGER NOT NULL DEFAULT 1
);`

const paymentsDDL = `
CREATE TABLE IF NOT EXISTS payments (
  id          TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  plan_code   TEXT NOT NULL,
  amount      INTEGER NOT NULL,
  months      INTEGER NOT NULL DEFAULT 0,
  status      TEXT NOT NULL,
  ref         TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  decided_at  TEXT NOT NULL DEFAULT ''
);`

const eventsDDL = `
CREATE TABLE IF NOT EXISTS events (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  at   TEXT NOT NULL,
  text TEXT NOT NULL
);`

// instancesDDL mengembalikan DDL tabel instances. Nama tabelnya bisa diganti
// supaya migrasi bisa menyiapkan tabel pengganti lebih dulu.
//
// kind: 'shared' = satu VPS melayani banyak pelanggan, 'dedicated' = satu VPS
// hanya untuk satu pelanggan.
func instancesDDL(table string) string {
	return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  id          TEXT PRIMARY KEY,
  token       TEXT NOT NULL,
  name        TEXT NOT NULL DEFAULT '',
  kind        TEXT NOT NULL DEFAULT 'shared' CHECK (kind IN ('shared', 'dedicated')),
  router_name TEXT NOT NULL DEFAULT '',
  version     TEXT NOT NULL DEFAULT '',
  last_seen   TEXT NOT NULL DEFAULT ''
);`, table)
}

// migrateLegacyInstances memindahkan instances.customer_id (satu instance satu
// pelanggan) ke customers.instance_id (satu instance banyak pelanggan).
//
// Baris instance lama dipertahankan apa adanya sebagai deployment
// kind='dedicated': id, token, router_name, version, dan last_seen disalin.
// Kolom name diisi institution pelanggan (atau namanya, atau id instance).
// session_name belum ada di data lama, jadi diturunkan dari institution
// pelanggan lewat sessionName ("Taufiq.net" menjadi "taufiq").
func migrateLegacyInstances(tx *sql.Tx) error {
	if _, err := tx.Exec(`UPDATE customers SET instance_id = COALESCE(
		(SELECT i.id FROM instances i WHERE i.customer_id = customers.id), '')`); err != nil {
		return err
	}

	// session_name dihitung di Go karena butuh normalisasi teks.
	type tenantName struct{ id, name string }
	var sessions []tenantName
	rows, err := tx.Query(`SELECT id, institution, pay_token FROM customers`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, institution, payToken string
		if err := rows.Scan(&id, &institution, &payToken); err != nil {
			rows.Close()
			return err
		}
		sessions = append(sessions, tenantName{id, sessionName(institution, payToken)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if _, err := tx.Exec(instancesDDL("instances_new")); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO instances_new (id, token, name, kind, router_name, version, last_seen)
		SELECT i.id, i.token,
		       COALESCE(NULLIF(TRIM(c.institution), ''), NULLIF(TRIM(c.name), ''), i.id),
		       'dedicated', i.router_name, i.version, i.last_seen
		  FROM instances i LEFT JOIN customers c ON c.id = i.customer_id`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE instances`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE instances_new RENAME TO instances`); err != nil {
		return err
	}

	for _, tn := range sessions {
		if _, err := tx.Exec(`UPDATE customers SET session_name = ? WHERE id = ?`, tn.name, tn.id); err != nil {
			return err
		}
	}
	return nil
}

// tableExists memeriksa apakah sebuah tabel sudah ada.
func tableExists(tx *sql.Tx, name string) (bool, error) {
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
	return n > 0, err
}

// columnExists memeriksa apakah sebuah kolom sudah ada di sebuah tabel.
func columnExists(tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, typ        string
			dflt             any
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// addColumn menambahkan kolom ke sebuah tabel kalau belum ada.
func addColumn(tx *sql.Tx, table, column, definition string) error {
	ok, err := columnExists(tx, table, column)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	_, err = tx.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

// queryer menyatukan *sql.DB, *sql.Tx, dan *sql.Conn supaya aturan yang sama
// bisa dijalankan di dalam maupun di luar transaksi tanpa dua salinan.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

/*
 * withImmediateTx menjalankan fn di dalam satu transaksi BEGIN IMMEDIATE.
 * Kunci tulis diambil sejak awal, jadi rangkaian baca-lalu-tulis tidak bisa
 * disusupi perubahan lain di sela-selanya. Ini aman karena pool-nya satu
 * koneksi (SetMaxOpenConns(1)): tidak ada operasi lain yang bisa menyelinap
 * di tengah transaksi.
 *
 * Koneksinya dipegang langsung, bukan lewat db.Begin, karena database/sql
 * memulai transaksi dengan BEGIN biasa; BEGIN IMMEDIATE harus ditulis sendiri.
 */
func (s *Store) withImmediateTx(fn func(q queryer) error) error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return err
	}
	if err := fn(conn); err != nil {
		_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		return err
	}
	_, err = conn.ExecContext(ctx, `COMMIT`)
	return err
}

// fallbackPlanCode adalah satu-satunya definisi paket cadangan: paket pertama
// yang masih dijual, sama dengan yang dipakai halaman pembayaran. Pelanggan
// yang belum pernah membayar dianggap memakai paket ini, jadi PlanUsage dan
// daftar pelanggan admin tidak bisa menghitungnya berbeda.
func fallbackPlanCode(q queryer) string {
	var code string
	err := q.QueryRowContext(context.Background(),
		`SELECT code FROM plans WHERE sale = 1 ORDER BY sort, price LIMIT 1`).Scan(&code)
	if err != nil {
		return ""
	}
	return code
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
	InstanceID  string // deployment yang menghosting pelanggan ini
	SessionName string // label sesi/subdomain di deployment itu, mis. "taufiq"
	PlanCode    string // paket terakhir yang dibayar
}

// Instance adalah satu pemasangan (deployment) panel Mikhmon di satu VPS.
// Satu deployment bisa melayani banyak pelanggan kalau kind='shared'.
type Instance struct {
	ID         string
	Token      string
	Name       string
	Kind       string // "shared" atau "dedicated"
	RouterName string
	Version    string
	LastSeen   string
}

// Claim adalah klaim pembayaran dari pelanggan. Months adalah durasi yang
// dibeli saat klaim dibuat; disimpan di klaim supaya perubahan paket
// sesudahnya tidak mengubah masa berlaku yang diterima pelanggan.
type Claim struct {
	ID          string
	CustomerID  string
	Customer    string
	Institution string
	PlanCode    string
	PlanLabel   string
	Amount      int64
	Months      int
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

// scanPlan membaca satu baris paket dari hasil kueri.
func scanPlan(row interface{ Scan(...any) error }) (Plan, error) {
	var p Plan
	err := row.Scan(&p.Code, &p.Label, &p.Months, &p.Price, &p.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// planFrom mengambil satu paket lewat queryer mana pun, supaya aturan paket
// bisa dijalankan juga di dalam transaksi.
func planFrom(q queryer, code string) (Plan, error) {
	return scanPlan(q.QueryRowContext(context.Background(),
		`SELECT code, label, months, price, note FROM plans WHERE code = ?`, code))
}

// PlanByCode mengambil satu paket.
func (s *Store) PlanByCode(code string) (Plan, error) { return planFrom(s.db, code) }

/* -------------------------------------------------------------- customers */

func (s *Store) scanCustomer(row interface{ Scan(...any) error }) (Customer, error) {
	var c Customer
	var sus int
	err := row.Scan(&c.ID, &c.Name, &c.Institution, &c.WA, &c.PayToken,
		&c.ValidFrom, &c.ValidUntil, &sus, &c.CreatedAt, &c.InstanceID, &c.SessionName)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	c.Suspended = sus != 0
	return c, err
}

const customerCols = `id, name, institution, wa, pay_token, valid_from, valid_until, suspended, created_at, instance_id, session_name`

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

// CustomerBySession mencari pelanggan dari deployment yang menghostingnya dan
// label sesinya (subdomain). Inilah cara panel bersama memilih pelanggan.
func (s *Store) CustomerBySession(instanceID, sessionName string) (Customer, error) {
	return s.scanCustomer(s.db.QueryRow(
		`SELECT `+customerCols+` FROM customers
		 WHERE instance_id = ? AND session_name = ? ORDER BY created_at, id LIMIT 1`,
		instanceID, sessionName))
}

// CustomersByInstance mengembalikan semua pelanggan yang dihosting satu
// deployment, terurut supaya hasilnya tidak berubah-ubah.
func (s *Store) CustomersByInstance(instanceID string) ([]Customer, error) {
	rows, err := s.db.Query(
		`SELECT `+customerCols+` FROM customers WHERE instance_id = ? ORDER BY session_name, id`, instanceID)
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

// InstanceByID mengambil deployment beserta token-nya (untuk heartbeat).
func (s *Store) InstanceByID(id string) (Instance, error) {
	var i Instance
	err := s.db.QueryRow(
		`SELECT id, token, name, kind, router_name, version, last_seen FROM instances WHERE id = ?`, id,
	).Scan(&i.ID, &i.Token, &i.Name, &i.Kind, &i.RouterName, &i.Version, &i.LastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return i, ErrNotFound
	}
	return i, err
}

// InstanceByCustomer mengambil deployment yang menghosting seorang pelanggan.
func (s *Store) InstanceByCustomer(customerID string) (Instance, error) {
	var i Instance
	err := s.db.QueryRow(
		`SELECT i.id, i.token, i.name, i.kind, i.router_name, i.version, i.last_seen
		   FROM instances i JOIN customers c ON c.instance_id = i.id
		  WHERE c.id = ?`, customerID,
	).Scan(&i.ID, &i.Token, &i.Name, &i.Kind, &i.RouterName, &i.Version, &i.LastSeen)
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

const claimCols = `p.id, p.customer_id, c.name, c.institution, p.plan_code, p.amount, p.months, p.status, p.ref, p.created_at`

func scanClaim(row interface{ Scan(...any) error }) (Claim, error) {
	var c Claim
	err := row.Scan(&c.ID, &c.CustomerID, &c.Customer, &c.Institution,
		&c.PlanCode, &c.Amount, &c.Months, &c.Status, &c.Ref, &c.CreatedAt)
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

	id := "p" + randHex(6)
	ref := "NOC-" + strings.ToUpper(randHex(2)) + "-" + strings.ToUpper(randHex(2))
	now := Now().Format(time.RFC3339)

	// Harga dan durasi disalin sekaligus di dalam INSERT, jadi paketnya tidak
	// bisa terhapus di sela pemeriksaan: kalau sudah tidak ada, SELECT-nya
	// tidak menghasilkan baris dan RowsAffected = 0.
	res, err := s.db.Exec(
		`INSERT INTO payments (id, customer_id, plan_code, amount, months, status, ref, created_at)
		 SELECT ?, ?, code, price, months, 'pending', ?, ? FROM plans WHERE code = ?`,
		id, customerID, ref, now, planCode)
	if err != nil {
		return Claim{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Claim{}, err
	}
	if n != 1 {
		return Claim{}, ErrNotFound
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
	// Durasi dibekukan di klaim (payments.months), jadi mengubah paket setelah
	// pelanggan mengklaim tidak mengubah berapa bulan yang dia dapat. Klaim
	// dari basis data lama belum menyimpan durasi, jadi untuk itu durasinya
	// masih diambil dari paket seperti semula. Paket yang sudah dihapus tidak
	// menghalangi penyetujuan selama durasinya tersimpan di klaim.
	months := claim.Months
	label := claim.PlanCode
	if plan, perr := s.PlanByCode(claim.PlanCode); perr == nil {
		label = plan.Label
		if months <= 0 {
			months = plan.Months
		}
	} else if months <= 0 {
		return time.Time{}, perr
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
	until := addMonths(from, months)

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
			claim.Institution, label, until.Format("2006-01-02"))); err != nil {
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

// Seed mengisi paket bawaan dan, kalau basis datanya masih kosong, contoh
// pemasangan bersama: satu panel dengan dua pelanggan, satu langganannya sehat
// dan satu sudah berakhir, supaya kedua keadaan itu langsung kelihatan.
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

	// Beginilah pemasangan bersama: satu panel VPS melayani dua pelanggan
	// dengan subdomain berbeda, jadi tidak perlu satu instance per pelanggan.
	instID := strings.ToUpper(randHex(6))
	if _, err := s.db.Exec(
		`INSERT INTO instances (id, token, name, kind, router_name, version, last_seen)
		 VALUES (?, ?, ?, 'shared', ?, '', '')`,
		instID, randHex(16), "Panel NOCIFY", "CHR-HOTSPOT"); err != nil {
		return err
	}

	// taufiq.nocify.id langganannya sehat (masih 26 hari), hendra.nocify.id
	// sudah lewat 10 hari supaya keadaan "berakhir" ikut terlihat.
	type demo struct {
		name, institution, wa, session string
		from, until                    time.Time
	}
	demos := []demo{
		{"Taufiq", "Taufiq.net", "6285139495106", "taufiq", today().AddDate(0, 0, -4), today().AddDate(0, 0, 26)},
		{"Hendra", "Hendra.net", "6281234567890", "hendra", today().AddDate(0, 0, -40), today().AddDate(0, 0, -10)},
	}
	for _, d := range demos {
		custID := "c" + randHex(6)
		if _, err := s.db.Exec(
			`INSERT INTO customers (id, name, institution, wa, pay_token, valid_from, valid_until, suspended, created_at, instance_id, session_name)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?)`,
			custID, d.name, d.institution, d.wa,
			"NOC-"+strings.ToUpper(randHex(2))+"-"+strings.ToUpper(randHex(2)),
			d.from.Format("2006-01-02"), d.until.Format("2006-01-02"),
			Now().Format(time.RFC3339), instID, d.session); err != nil {
			return err
		}
		// Pembayaran pertama yang sudah disetujui, supaya catatannya utuh: tanpa
		// ini pelanggan punya tanggal berlaku tetapi tidak punya paket yang bisa
		// dibaca.
		paid := Now().AddDate(0, 0, -26).Format(time.RFC3339)
		if _, err := s.db.Exec(
			`INSERT INTO payments (id, customer_id, plan_code, amount, months, status, ref, created_at, decided_at)
			 VALUES (?, ?, 'P1M', 50000, 1, 'approved', ?, ?, ?)`,
			"p"+randHex(6), custID,
			"NOC-"+strings.ToUpper(randHex(2))+"-"+strings.ToUpper(randHex(2)),
			paid, paid); err != nil {
			return err
		}
	}
	return s.LogEvent("Portal disiapkan. Satu panel contoh berisi dua pelanggan (satu aktif, satu berakhir).")
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

// sessionName menurunkan label sesi/subdomain dari data pelanggan. Sumber
// pertama institution, karena institution biasanya sudah berupa nama domain
// pelanggan: "Taufiq.net" menjadi "taufiq". Kalau institution kosong atau tidak
// menyisakan huruf/angka, pay_token yang dipakai, dan sebagai jalan terakhir
// dipakai "customer".
func sessionName(institution, payToken string) string {
	if v := slugLabel(institution); v != "" {
		return v
	}
	if v := slugLabel(payToken); v != "" {
		return v
	}
	return "customer"
}

// slugLabel membersihkan teks menjadi label yang aman dipakai sebagai nama
// subdomain: huruf kecil, hanya huruf/angka/tanda hubung. Bagian setelah titik
// pertama dibuang, jadi "taufiq.nocify.id" menjadi "taufiq".
func slugLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '-' || r == '_' || r == ' ':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}
