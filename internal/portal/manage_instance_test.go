package portal

import (
	"net/http"
	"testing"
)

/*
 * Nama panel dipakai pemilik portal untuk membedakan panel yang satu dari yang
 * lain. Panel yang mendaftar dari baris perintah tidak punya alamat web, jadi
 * namanya jatuh ke "Panel tanpa nama" - dan portal dengan dua panel bernama sama
 * tidak bisa dipakai. Tes di sini menjaga dua hal: nama bawaan itu bisa
 * dilengkapi sendiri oleh panelnya, dan nama yang dipilih manusia tidak pernah
 * ditimpa.
 */

func TestEnrollTanpaHostDiberiNamaBawaan(t *testing.T) {
	st := openTestStore(t)

	inst, err := st.EnrollInstance("", "3.20")
	if err != nil {
		t.Fatalf("EnrollInstance: %v", err)
	}
	if inst.Name != panelTanpaNama {
		t.Fatalf("nama = %q, mau %q", inst.Name, panelTanpaNama)
	}

	// Daftar lagi dengan host kosong harus mengembalikan panel yang sama, bukan
	// membuat panel kedua yang juga tanpa nama.
	lagi, err := st.EnrollInstance("", "3.20")
	if err != nil {
		t.Fatalf("EnrollInstance kedua: %v", err)
	}
	if lagi.ID != inst.ID {
		t.Errorf("pendaftaran ulang membuat panel baru: %s != %s", lagi.ID, inst.ID)
	}
	if n := countRows(t, st, `SELECT COUNT(*) FROM instances`); n != 1 {
		t.Errorf("jumlah panel = %d, mau 1", n)
	}
}

func TestNamaPanelDilengkapiDariLaporan(t *testing.T) {
	st := openTestStore(t)
	inst, err := st.EnrollInstance("", "3.20")
	if err != nil {
		t.Fatalf("EnrollInstance: %v", err)
	}

	// Laporan dari browser membawa alamat panelnya.
	ganti, err := st.NameInstance(inst.ID, "taufiq.nocify.id")
	if err != nil {
		t.Fatalf("NameInstance: %v", err)
	}
	if !ganti {
		t.Fatal("nama bawaan tidak dilengkapi")
	}
	sesudah, err := st.InstanceByID(inst.ID)
	if err != nil {
		t.Fatalf("InstanceByID: %v", err)
	}
	if sesudah.Name != "taufiq.nocify.id" {
		t.Fatalf("nama = %q, mau taufiq.nocify.id", sesudah.Name)
	}

	// Laporan berikutnya tidak boleh mengubah nama itu lagi.
	if ganti, err := st.NameInstance(inst.ID, "nama.lain.nocify.id"); err != nil || ganti {
		t.Errorf("nama yang sudah benar ikut berubah (ganti=%v, err=%v)", ganti, err)
	}
	if cek, _ := st.InstanceByID(inst.ID); cek.Name != "taufiq.nocify.id" {
		t.Errorf("nama berubah menjadi %q", cek.Name)
	}

	// Host kosong tidak mengubah apa pun.
	if ganti, err := st.NameInstance(inst.ID, "   "); err != nil || ganti {
		t.Errorf("host kosong mengubah nama (ganti=%v, err=%v)", ganti, err)
	}

	// Id yang tidak dikenal harus dilaporkan sebagai galat, bukan diam-diam.
	if _, err := st.NameInstance("tidak-ada", "x.nocify.id"); err == nil {
		t.Error("id yang tidak dikenal tidak dilaporkan sebagai galat")
	}
}

func TestNamaPanelPilihanManusiaTidakDitimpa(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	// Panel contoh sudah punya nama yang dipilih manusia ("Panel NOCIFY").
	inst := seedInstance(t, st)

	ganti, err := st.NameInstance(inst.ID, "taufiq.nocify.id")
	if err != nil {
		t.Fatalf("NameInstance: %v", err)
	}
	if ganti {
		t.Error("nama panel yang sudah dipilih manusia ditimpa")
	}
	sesudah, _ := st.InstanceByID(inst.ID)
	if sesudah.Name != inst.Name {
		t.Errorf("nama = %q, mau tetap %q", sesudah.Name, inst.Name)
	}
}

func TestHeartbeatMelengkapiNamaPanel(t *testing.T) {
	st := openTestStore(t)
	if err := st.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	// Panel yang mendaftar dari baris perintah: tanpa nama, lalu dipakai
	// pelanggan supaya heartbeat-nya punya tenant yang dikenal.
	inst, err := st.EnrollInstance("", "3.20")
	if err != nil {
		t.Fatalf("EnrollInstance: %v", err)
	}
	if _, err := st.CreateCustomer(NewCustomer{
		Name: "Taufiq", Institution: "Taufiq.net", SessionName: "taufiq",
		InstanceID: inst.ID, ValidUntil: Now().AddDate(0, 0, 30).Format("2006-01-02"),
	}); err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}

	h := NewServer(Config{BaseURL: "http://portal.test"}, st).Handler()

	// Heartbeat pertama membawa alamat panelnya.
	code, body := doHeartbeat(t, h, map[string]any{
		"instance_id": inst.ID, "token": inst.Token, "version": "3.20",
		"tenant": "taufiq", "host": "taufiq.nocify.id",
	})
	if code != http.StatusOK {
		t.Fatalf("heartbeat: status %d, mau 200 (%v)", code, body)
	}
	sesudah, err := st.InstanceByID(inst.ID)
	if err != nil {
		t.Fatalf("InstanceByID: %v", err)
	}
	if sesudah.Name != "taufiq.nocify.id" {
		t.Fatalf("nama = %q, mau taufiq.nocify.id", sesudah.Name)
	}

	// Heartbeat dari panel versi lama (tidak mengirim host) tetap jalan seperti
	// biasa, dan namanya tidak hilang.
	code, body = doHeartbeat(t, h, map[string]any{
		"instance_id": inst.ID, "token": inst.Token, "tenant": "taufiq",
	})
	if code != http.StatusOK || body["state"] != "active" {
		t.Fatalf("heartbeat tanpa host: status %d, state %v (%v)", code, body["state"], body)
	}
	if cek, _ := st.InstanceByID(inst.ID); cek.Name != "taufiq.nocify.id" {
		t.Errorf("nama hilang setelah heartbeat tanpa host: %q", cek.Name)
	}

	// Token yang salah tetap ditolak, dan tidak boleh sempat mengganti nama.
	code, _ = doHeartbeat(t, h, map[string]any{
		"instance_id": inst.ID, "token": "salah", "tenant": "taufiq", "host": "jahat.nocify.id",
	})
	if code != http.StatusUnauthorized {
		t.Fatalf("token salah: status %d, mau 401", code)
	}
	if cek, _ := st.InstanceByID(inst.ID); cek.Name != "taufiq.nocify.id" {
		t.Errorf("nama diganti oleh permintaan tanpa token sah: %q", cek.Name)
	}
}
