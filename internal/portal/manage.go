package portal

import (
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

	validFrom, validUntil := "", ""
	if in.PlanCode != "" {
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
