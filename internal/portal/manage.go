package portal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

/*
 * Pengelolaan pelanggan dari halaman admin portal.
 *
 * Isinya operasi yang dipakai admin sehari-hari: membuat pemasangan panel,
 * menambah pelanggan, mengubah datanya, memperpanjang tanpa lewat pembayaran,
 * dan menghapusnya. Aturan validasinya ditaruh di sini supaya penangan HTTP
 * tinggal menerjemahkan galatnya jadi kode balasan.
 */

// ErrInvalid menandai masukan yang tidak sah, supaya penangan HTTP bisa
// membalas 400 beserta alasannya alih-alih 500.
type ErrInvalid struct{ Msg string }

func (e ErrInvalid) Error() string { return e.Msg }

func invalid(format string, a ...any) error {
	return ErrInvalid{Msg: fmt.Sprintf(format, a...)}
}

/*
 * NormalizeSessionName memaksa nama sesi jadi satu label subdomain yang aman:
 * huruf kecil, angka, dan tanda hubung saja. Nilai ini dipakai sebagai
 * <nama>.nocify.id sekaligus nama berkas sesi di panel, jadi tidak boleh
 * mengandung apa pun selain itu.
 */
func NormalizeSessionName(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	var b strings.Builder
	dash := false
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '-' || r == ' ' || r == '_' || r == '.':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	return out
}

// NewCustomer adalah masukan untuk membuat pelanggan baru.
type NewCustomer struct {
	Name        string
	Institution string
	WA          string
	SessionName string
	InstanceID  string
	PlanCode    string // opsional: langsung memberi masa berlaku
	ValidUntil  string // opsional: YYYY-MM-DD, dipakai saat mendaftarkan pelanggan lama
}

// CustomerEdit adalah perubahan data pelanggan yang diizinkan.
type CustomerEdit struct {
	Name        string
	Institution string
	WA          string
	SessionName string
	InstanceID  string
}

// InstanceUsage adalah satu pemasangan panel beserta jumlah pelanggannya.
type InstanceUsage struct {
	Instance
	Customers int
}

// Instances mengembalikan semua pemasangan panel, terurut menurut urutan dibuat.
func (s *Store) Instances() ([]InstanceUsage, error) {
	rows, err := s.db.Query(
		`SELECT i.id, i.token, i.name, i.kind, i.router_name, i.version, i.last_seen,
		        (SELECT COUNT(*) FROM customers c WHERE c.instance_id = i.id)
		   FROM instances i ORDER BY i.rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InstanceUsage
	for rows.Next() {
		var u InstanceUsage
		if err := rows.Scan(&u.ID, &u.Token, &u.Name, &u.Kind, &u.RouterName,
			&u.Version, &u.LastSeen, &u.Customers); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

/*
 * CreateInstance membuat pemasangan panel baru dan mengembalikan token-nya.
 * Token ini yang dipasang di include/instance.php pada VPS panel, supaya
 * panelnya bisa melapor ke portal. Ditampilkan sekali di halaman admin.
 */
func (s *Store) CreateInstance(name, kind string) (Instance, error) {
	if kind != "shared" && kind != "dedicated" {
		kind = "shared"
	}
	inst := Instance{
		ID:    strings.ToUpper(randHex(6)),
		Token: randHex(16),
		Name:  strings.TrimSpace(name),
		Kind:  kind,
	}
	if inst.Name == "" {
		inst.Name = "Panel " + inst.ID[:4]
	}
	if _, err := s.db.Exec(
		`INSERT INTO instances (id, token, name, kind, router_name, version, last_seen)
		 VALUES (?, ?, ?, ?, '', '', '')`,
		inst.ID, inst.Token, inst.Name, inst.Kind); err != nil {
		return Instance{}, err
	}
	_ = s.LogEvent("Pemasangan panel " + inst.Name + " dibuat.")
	return inst, nil
}

// CreateCustomer membuat pelanggan baru beserta token halaman pembayarannya.
func (s *Store) CreateCustomer(in NewCustomer) (Customer, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Institution = strings.TrimSpace(in.Institution)
	in.WA = strings.TrimSpace(in.WA)
	in.SessionName = NormalizeSessionName(in.SessionName)

	if in.Name == "" {
		return Customer{}, invalid("Nama pelanggan belum diisi.")
	}
	if in.Institution == "" {
		return Customer{}, invalid("Nama usaha belum diisi.")
	}
	if in.SessionName == "" {
		return Customer{}, invalid("Subdomain belum diisi atau isinya tidak sah. " +
			"Pakai huruf kecil, angka, dan tanda hubung.")
	}
	if _, err := s.InstanceByID(in.InstanceID); err != nil {
		return Customer{}, invalid("Panel yang dipilih tidak dikenal.")
	}
	if _, err := s.CustomerBySession(in.InstanceID, in.SessionName); err == nil {
		return Customer{}, invalid("Subdomain itu sudah dipakai pelanggan lain di panel ini.")
	} else if !errors.Is(err, ErrNotFound) {
		return Customer{}, err
	}

	/*
	 * Masa berlaku bisa diberikan dua cara: lewat paket (dihitung dari hari
	 * ini), atau lewat tanggal berakhir yang disebut langsung. Yang kedua
	 * dipakai saat mendaftarkan pelanggan lama ke portal, supaya tanggalnya
	 * bisa disamakan dengan yang sudah berjalan di panel.
	 */
	validFrom, validUntil := "", ""
	if in.ValidUntil != "" {
		until, err := time.ParseInLocation("2006-01-02", in.ValidUntil, loc)
		if err != nil {
			return Customer{}, invalid("Tanggal berakhir tidak sah. Pakai format YYYY-MM-DD.")
		}
		validFrom = today().Format("2006-01-02")
		validUntil = until.Format("2006-01-02")
	} else if in.PlanCode != "" {
		plan, err := s.PlanByCode(in.PlanCode)
		if err != nil {
			return Customer{}, invalid("Paket tidak dikenal.")
		}
		from := today()
		validFrom = from.Format("2006-01-02")
		validUntil = addMonths(from, plan.Months).Format("2006-01-02")
	}

	id := "c" + randHex(6)
	now := Now().Format(time.RFC3339)

	// Token halaman pembayaran dibuat acak; kalau kebetulan bentrok, coba lagi.
	for try := 0; try < 5; try++ {
		payToken := "NOC-" + strings.ToUpper(randHex(2)) + "-" + strings.ToUpper(randHex(2))
		_, err := s.db.Exec(
			`INSERT INTO customers
			   (id, name, institution, wa, pay_token, valid_from, valid_until, suspended, created_at, instance_id, session_name)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?)`,
			id, in.Name, in.Institution, in.WA, payToken,
			validFrom, validUntil, now, in.InstanceID, in.SessionName)
		if err == nil {
			_ = s.LogEvent(fmt.Sprintf("Pelanggan %s (%s) dibuat.", in.Institution, in.SessionName))
			return s.CustomerByID(id)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Customer{}, err
		}
	}
	return Customer{}, fmt.Errorf("gagal membuat token pembayaran yang unik")
}

// UpdateCustomer mengubah data pelanggan. Mengganti subdomain berarti sesi di
// panel juga harus dinamai ulang, jadi pemakainya perlu diberi tahu.
func (s *Store) UpdateCustomer(id string, in CustomerEdit) error {
	cust, err := s.CustomerByID(id)
	if err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Institution = strings.TrimSpace(in.Institution)
	in.WA = strings.TrimSpace(in.WA)
	in.SessionName = NormalizeSessionName(in.SessionName)

	if in.Name == "" {
		return invalid("Nama pelanggan belum diisi.")
	}
	if in.Institution == "" {
		return invalid("Nama usaha belum diisi.")
	}
	if in.SessionName == "" {
		return invalid("Subdomain belum diisi atau isinya tidak sah.")
	}
	if in.InstanceID == "" {
		in.InstanceID = cust.InstanceID
	}
	if _, err := s.InstanceByID(in.InstanceID); err != nil {
		return invalid("Panel yang dipilih tidak dikenal.")
	}
	if other, err := s.CustomerBySession(in.InstanceID, in.SessionName); err == nil && other.ID != id {
		return invalid("Subdomain itu sudah dipakai pelanggan lain di panel ini.")
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}

	if _, err := s.db.Exec(
		`UPDATE customers SET name = ?, institution = ?, wa = ?, session_name = ?, instance_id = ?
		  WHERE id = ?`,
		in.Name, in.Institution, in.WA, in.SessionName, in.InstanceID, id); err != nil {
		return err
	}
	_ = s.LogEvent(fmt.Sprintf("Data pelanggan %s diubah.", in.Institution))
	return nil
}

/*
 * ExtendCustomer memperpanjang langganan tanpa lewat pembayaran, untuk
 * pelanggan yang membayar tunai atau transfer di luar QRIS. Sisa waktu yang
 * masih berjalan tidak hangus, sama seperti saat menyetujui klaim.
 */
func (s *Store) ExtendCustomer(id string, months int) (time.Time, error) {
	if months < 1 || months > 36 {
		return time.Time{}, invalid("Lama perpanjangan harus antara 1 dan 36 bulan.")
	}
	cust, err := s.CustomerByID(id)
	if err != nil {
		return time.Time{}, err
	}

	from := today()
	if cust.ValidUntil != "" {
		if until, perr := time.ParseInLocation("2006-01-02", cust.ValidUntil, loc); perr == nil && until.After(from) {
			from = until
		}
	}
	until := addMonths(from, months)

	if _, err := s.db.Exec(
		`UPDATE customers SET valid_from = ?, valid_until = ?, suspended = 0 WHERE id = ?`,
		from.Format("2006-01-02"), until.Format("2006-01-02"), id); err != nil {
		return time.Time{}, err
	}
	_ = s.LogEvent(fmt.Sprintf("Langganan %s diperpanjang %d bulan sampai %s.",
		cust.Institution, months, until.Format("2006-01-02")))
	return until, nil
}

// DeleteCustomer menghapus pelanggan beserta klaim pembayarannya.
func (s *Store) DeleteCustomer(id string) error {
	cust, err := s.CustomerByID(id)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM customers WHERE id = ?`, id); err != nil {
		return err
	}
	_ = s.LogEvent(fmt.Sprintf("Pelanggan %s dihapus.", cust.Institution))
	return nil
}

// DeleteInstance menghapus pemasangan panel. Ditolak kalau masih ada pelanggan
// di dalamnya, supaya tidak ada pelanggan yang menggantung tanpa panel.
func (s *Store) DeleteInstance(id string) error {
	inst, err := s.InstanceByID(id)
	if err != nil {
		return err
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM customers WHERE instance_id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return invalid("Panel ini masih menangani %d pelanggan. "+
			"Pindahkan atau hapus pelanggannya dulu.", n)
	}
	if _, err := s.db.Exec(`DELETE FROM instances WHERE id = ?`, id); err != nil {
		return err
	}
	_ = s.LogEvent("Pemasangan panel " + inst.Name + " dihapus.")
	return nil
}

// panelTanpaNama adalah nama bawaan untuk panel yang mendaftar tanpa menyebut
// namanya sama sekali. Nama ini boleh diganti sendiri oleh panelnya begitu ia
// melapor dari permintaan web yang membawa alamatnya.
const panelTanpaNama = "Panel tanpa nama"

// namaPanelBawaan melaporkan apakah sebuah nama masih nama bawaan, jadi masih
// boleh dilengkapi panelnya sendiri. Nama yang dipilih manusia tidak pernah
// ditimpa.
func namaPanelBawaan(name string) bool {
	return strings.TrimSpace(name) == "" || strings.EqualFold(strings.TrimSpace(name), panelTanpaNama)
}

// rapikanNamaPanel menyiapkan nama panel dari laporan panelnya.
func rapikanNamaPanel(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

// EnrollInstance mendaftarkan pemasangan panel yang baru memasang dirinya.
//
// Dipakai supaya tidak ada berkas yang harus diisi tangan di sisi panel:
// panelnya melapor sekali, portal membuatkan identitas untuknya, lalu identitas
// itu disimpan panel sendiri. Dijalankan berulang aman - nama yang sama akan
// mengembalikan instance yang sudah ada, bukan membuat yang kedua.
func (s *Store) EnrollInstance(host, version string) (Instance, error) {
	name := rapikanNamaPanel(host)
	if name == "" {
		name = panelTanpaNama
	}

	// Sudah pernah mendaftar dengan nama yang sama?
	var ada Instance
	err := s.db.QueryRow(
		`SELECT id, token, name, kind, router_name, version, last_seen
		   FROM instances WHERE name = ? ORDER BY rowid LIMIT 1`, name,
	).Scan(&ada.ID, &ada.Token, &ada.Name, &ada.Kind, &ada.RouterName, &ada.Version, &ada.LastSeen)
	if err == nil {
		if version != "" {
			_, _ = s.db.Exec(`UPDATE instances SET version = ? WHERE id = ?`, version, ada.ID)
			ada.Version = version
		}
		return ada, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Instance{}, err
	}

	inst := Instance{
		ID:      strings.ToUpper(randHex(6)),
		Token:   randHex(16),
		Name:    name,
		Kind:    "shared",
		Version: strings.TrimSpace(version),
	}
	if _, err := s.db.Exec(
		`INSERT INTO instances (id, token, name, kind, router_name, version, last_seen)
		 VALUES (?, ?, ?, ?, '', ?, '')`,
		inst.ID, inst.Token, inst.Name, inst.Kind, inst.Version); err != nil {
		return Instance{}, err
	}
	_ = s.LogEvent("Panel " + inst.Name + " mendaftar sendiri.")
	return inst, nil
}

/*
 * NameInstance melengkapi nama panel dari laporan heartbeat-nya.
 *
 * Kenapa perlu: panel yang mendaftar sendiri lewat baris perintah tidak punya
 * alamat web (HTTP_HOST kosong), jadi namanya jatuh ke "Panel tanpa nama" -
 * pemilik portal lalu melihat dua panel dan tidak tahu mana yang benar. Laporan
 * heartbeat berikutnya datang dari browser, yang tentu tahu alamatnya sendiri,
 * jadi namanya bisa dilengkapi tanpa perlu ada yang mengisi apa pun.
 *
 * Nama yang sudah dipilih manusia TIDAK pernah ditimpa: fungsi ini hanya bekerja
 * selagi namanya masih nama bawaan.
 */
func (s *Store) NameInstance(id, name string) (bool, error) {
	name = rapikanNamaPanel(name)
	if name == "" {
		return false, nil
	}

	var sekarang string
	err := s.db.QueryRow(`SELECT name FROM instances WHERE id = ?`, id).Scan(&sekarang)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if !namaPanelBawaan(sekarang) || strings.EqualFold(sekarang, name) {
		return false, nil
	}

	if _, err := s.db.Exec(`UPDATE instances SET name = ? WHERE id = ?`, name, id); err != nil {
		return false, err
	}
	_ = s.LogEvent(fmt.Sprintf("Panel yang belum bernama sekarang bernama %s.", name))
	return true, nil
}

/* ------------------------------------------------------------------ paket */

/*
 * Paket sewa (plans) dulu hanya diisi nilai awal waktu basis data dibuat,
 * sehingga harga tidak bisa diubah tanpa menyentuh kode. Fungsi di bawah ini
 * yang membuat harga bisa diubah dari halaman admin.
 *
 * Kode paket sengaja tidak bisa diganti setelah dibuat: kode itu dirujuk oleh
 * Kode paket sengaja tidak bisa diganti setelah dibuat: kode itu dirujuk oleh
 * riwayat pembayaran (payments.plan_code), jadi menggantinya akan membuat
 * pembayaran yang sudah tercatat menunjuk paket yang salah.
 * menggantinya akan membuat riwayat pembayaran menunjuk paket yang salah.
 */

// ErrInUse menandai data yang masih dipakai data lain, mis. paket yang masih
// dipakai pelanggan. Penangan HTTP menerjemahkannya jadi 409 supaya antarmuka
// bisa membedakannya dari masukan yang salah (400).
type ErrInUse struct{ Msg string }

func (e ErrInUse) Error() string { return e.Msg }

// PlanInput adalah paket yang dikirim dari halaman admin.
type PlanInput struct {
	Code   string
	Label  string
	Months int
	Price  int64
	Note   string
}

/*
 * NormalizePlanCode memaksa kode paket jadi huruf besar yang aman dipakai
 * sebagai kunci basis data dan sebagai parameter di halaman pembayaran.
 */
func NormalizePlanCode(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 16 {
		out = strings.Trim(out[:16], "-")
	}
	return out
}

// maxPlanPrice menjaga salah ketik (mis. 1000000000000) tidak lolos.
const maxPlanPrice = 1000000000

// validatePlan menaruh semua aturan paket di satu tempat supaya pembuatan dan
// pengubahan memakai aturan yang sama.
func validatePlan(in PlanInput) (PlanInput, error) {
	in.Code = NormalizePlanCode(in.Code)
	in.Label = strings.TrimSpace(in.Label)
	in.Note = strings.TrimSpace(in.Note)

	if in.Code == "" {
		return in, invalid("Kode paket belum diisi atau isinya tidak sah.")
	}
	if in.Label == "" {
		return in, invalid("Nama paket belum diisi.")
	}
	if len([]rune(in.Label)) > 40 {
		return in, invalid("Nama paket terlalu panjang, maksimal 40 huruf.")
	}
	if len([]rune(in.Note)) > 40 {
		return in, invalid("Catatan terlalu panjang, maksimal 40 huruf.")
	}
	if in.Months < 1 || in.Months > 36 {
		return in, invalid("Durasi harus antara 1 dan 36 bulan.")
	}
	if in.Price < 1 {
		return in, invalid("Harga harus lebih dari 0.")
	}
	if in.Price > maxPlanPrice {
		return in, invalid("Harga terlalu besar, maksimal Rp 1.000.000.000.")
	}
	return in, nil
}

// PlanUsage menghitung berapa pelanggan yang memakai tiap paket.
func (s *Store) PlanUsage() (map[string]int, error) {
	return planUsage(s.db)
}

// planUsage menjalankan perhitungan yang sama di dalam transaksi, supaya
// penghapusan paket memakai angka yang tidak berubah di sela-selanya.
func planUsage(q queryer) (map[string]int, error) {
	// Pelanggan tidak menyimpan kode paket di tabelnya sendiri: paket yang
	// berlaku adalah paket dari pembayaran terakhir yang disetujui, sama
	// seperti yang dipakai halaman admin dan halaman pembayaran. Pelanggan
	// yang belum pernah membayar dihitung ke paket cadangan yang sama dengan
	// lastPlanCode; tanpa itu paket cadangan tampak kosong padahal daftar
	// pelanggan admin menunjukkan mereka memakai paket itu.
	rows, err := q.QueryContext(context.Background(), `
		SELECT COALESCE(
		         NULLIF((SELECT p.plan_code FROM payments p
		                  WHERE p.customer_id = c.id AND p.status = 'approved'
		                  ORDER BY p.decided_at DESC LIMIT 1), ''),
		         ?) AS kode,
		       COUNT(*)
		  FROM customers c
		 GROUP BY kode`, fallbackPlanCode(q))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		out[code] = n
	}
	return out, rows.Err()
}

// SavePlan membuat paket baru.
func (s *Store) SavePlan(in PlanInput) (Plan, error) {
	in, err := validatePlan(in)
	if err != nil {
		return Plan{}, err
	}
	if _, err := s.PlanByCode(in.Code); err == nil {
		return Plan{}, invalid("Kode paket %s sudah dipakai paket lain.", in.Code)
	} else if !errors.Is(err, ErrNotFound) {
		return Plan{}, err
	}

	// Paket baru ditaruh paling bawah supaya urutan paket yang sudah ada tidak
	// berubah di halaman pembayaran pelanggan.
	var sort int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(sort), -1) + 1 FROM plans`).Scan(&sort); err != nil {
		return Plan{}, err
	}
	if _, err := s.db.Exec(
		`INSERT INTO plans (code, label, months, price, note, sort, sale)
		 VALUES (?, ?, ?, ?, ?, ?, 1)`,
		in.Code, in.Label, in.Months, in.Price, in.Note, sort); err != nil {
		// Pemeriksaan PlanByCode di atas bisa dilewati dua permintaan yang
		// datang hampir bersamaan; saat itu PRIMARY KEY di basis data yang
		// menolak. Pesannya diterjemahkan supaya admin melihat sebabnya
		// sebagai masukan yang salah (400), bukan 500.
		if isUniqueViolation(err) {
			return Plan{}, invalid("Kode paket %s sudah dipakai paket lain.", in.Code)
		}
		return Plan{}, err
	}
	_ = s.LogEvent(fmt.Sprintf("Paket %s dibuat dengan harga %d.", in.Code, in.Price))
	return s.PlanByCode(in.Code)
}

// isUniqueViolation mendeteksi pelanggaran UNIQUE/PRIMARY KEY dari driver
// modernc. Tidak ada tipe galat khusus yang bisa dipakai, jadi teks galatnya
// yang diperiksa; itu cukup untuk menerjemahkan bentrok kode paket jadi 400.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "primary key")
}

// UpdatePlan mengubah paket yang sudah ada. Kode paket tidak ikut diubah.
func (s *Store) UpdatePlan(code string, in PlanInput) (Plan, error) {
	code = NormalizePlanCode(code)
	old, err := s.PlanByCode(code)
	if err != nil {
		return Plan{}, err
	}

	in.Code = code
	in, err = validatePlan(in)
	if err != nil {
		return Plan{}, err
	}
	if _, err := s.db.Exec(
		`UPDATE plans SET label = ?, months = ?, price = ?, note = ? WHERE code = ?`,
		in.Label, in.Months, in.Price, in.Note, code); err != nil {
		return Plan{}, err
	}

	if old.Price != in.Price {
		_ = s.LogEvent(fmt.Sprintf("Harga paket %s diubah dari %d ke %d.", code, old.Price, in.Price))
	} else {
		_ = s.LogEvent(fmt.Sprintf("Paket %s diubah.", code))
	}
	return s.PlanByCode(code)
}

/*
 * DeletePlan menghapus paket. Pemeriksaan dan penghapusan dijalankan dalam
 * satu transaksi BEGIN IMMEDIATE, supaya tidak ada klaim baru yang dibuat
 * atau disetujui di sela pemeriksaan dan paketnya terhapus padahal masih
 * dirujuk.
 *
 * Ada tiga hal yang menahan penghapusan: pelanggan yang memakai paket itu,
 * pembayaran yang belum diputus, dan paket itu satu-satunya yang dijual.
 * Karena portal belum punya cara memindahkan paket pelanggan, admin
 * diberitahu bahwa pelanggannya harus membeli paket lain lebih dulu.
 */
func (s *Store) DeletePlan(code string) error {
	code = NormalizePlanCode(code)

	err := s.withImmediateTx(func(q queryer) error {
		plan, err := planFrom(q, code)
		if err != nil {
			return err
		}

		usage, err := planUsage(q)
		if err != nil {
			return err
		}
		if n := usage[code]; n > 0 {
			return ErrInUse{Msg: fmt.Sprintf(
				"Paket %s masih dipakai %d pelanggan, jadi belum bisa dihapus. "+
					"Pelanggan itu harus membeli paket lain lebih dulu.", plan.Label, n)}
		}

		// Pembayaran yang belum diputus masih merujuk paket ini, jadi paketnya
		// tidak boleh hilang sebelum klaimnya jelas: setujui (durasi yang
		// dibeli sudah tersimpan di klaim) atau tolak.
		var menunggu int
		if err := q.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM payments WHERE plan_code = ? AND status = 'pending'`, code).Scan(&menunggu); err != nil {
			return err
		}
		if menunggu > 0 {
			return ErrInUse{Msg: fmt.Sprintf(
				"Paket %s masih menunggu %d pembayaran yang belum diputus, jadi belum bisa dihapus. "+
					"Putuskan dulu pembayarannya, disetujui atau ditolak.", plan.Label, menunggu)}
		}

		// Setidaknya harus tersisa satu paket yang dijual: dari daftar itu
		// pelanggan baru dan pelanggan yang belum pernah membayar mengambil
		// paketnya, jadi menghapus yang terakhir membuat semuanya buntu.
		var sisa int
		if err := q.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM plans WHERE sale = 1 AND code <> ?`, code).Scan(&sisa); err != nil {
			return err
		}
		if sisa == 0 {
			return ErrInUse{Msg: fmt.Sprintf(
				"Paket %s adalah paket terakhir yang dijual, jadi belum bisa dihapus. "+
					"Minimal harus ada satu paket, kalau tidak pelanggan tidak bisa memperpanjang sama sekali.",
				plan.Label)}
		}

		_, err = q.ExecContext(context.Background(), `DELETE FROM plans WHERE code = ?`, code)
		return err
	})
	if err != nil {
		return err
	}
	_ = s.LogEvent(fmt.Sprintf("Paket %s dihapus.", code))
	return nil
}
