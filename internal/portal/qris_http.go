package portal

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

/*
 * Unggah gambar QRIS dari halaman admin.
 *
 * Sebelumnya berkasnya harus ditaruh manual di server, dan itu merepotkan
 * untuk sesuatu yang mungkin diganti sesekali. Berkasnya disimpan di samping
 * basis data (bukan di dalam image container) supaya ikut terbawa saat
 * container dibangun ulang, dan supaya tidak perlu mount tambahan.
 */

// maxQRISBytes adalah batas ukuran unggahan.
const maxQRISBytes = 2 << 20 // 2 MB

// qrisExt memetakan tipe isi yang dikenali ke ekstensi berkas.
var qrisExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// qrisBase mengembalikan jalur gambar QRIS tanpa ekstensi.
// Contoh: ./data/qris.png -> ./data/qris
func (s *Server) qrisBase() string {
	p := strings.TrimSpace(s.cfg.QRISImage)
	if p == "" {
		p = "./data/qris.png"
	}
	ext := filepath.Ext(p)
	if ext != "" {
		p = strings.TrimSuffix(p, ext)
	}
	return p
}

/*
 * qrisFile mencari gambar QRIS yang sedang dipakai.
 * Ekstensinya bisa berubah mengikuti berkas yang terakhir diunggah, jadi yang
 * dicari adalah nama dasarnya dengan ekstensi yang diizinkan.
 */
func (s *Server) qrisFile() string {
	base := s.qrisBase()
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
		if st, err := os.Stat(base + ext); err == nil && !st.IsDir() {
			return base + ext
		}
	}
	// Hormati juga nama persis dari pengaturan, kalau ada.
	if p := strings.TrimSpace(s.cfg.QRISImage); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// qrisURL adalah alamat yang dipakai halaman pembayaran. Diberi penanda waktu
// berkas supaya gambar baru langsung terlihat, tanpa bergantung pada aturan
// cache di peramban.
func (s *Server) qrisURL() string {
	p := s.qrisFile()
	if p == "" {
		return ""
	}
	st, err := os.Stat(p)
	if err != nil {
		return "/qris.png"
	}
	return "/qris.png?v=" + itoa(st.ModTime().Unix())
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// handleQRISImage menyajikan gambar QRIS yang sedang dipakai.
func (s *Server) handleQRISImage(w http.ResponseWriter, r *http.Request) {
	p := s.qrisFile()
	if p == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, p)
}

// handleQRISUpload menerima gambar QRIS baru dari halaman admin.
func (s *Server) handleQRISUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxQRISBytes+512)
	if err := r.ParseMultipartForm(maxQRISBytes); err != nil {
		writeInvalid(w, invalid("Berkasnya terlalu besar. Maksimal 2 MB."))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeInvalid(w, invalid("Tidak ada berkas yang dikirim."))
		return
	}
	defer file.Close()

	// Sniff tipe isinya, jangan percaya ekstensi atau header dari peramban.
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		writeInvalid(w, invalid("Berkasnya tidak bisa dibaca."))
		return
	}
	head = head[:n]

	ext, ok := qrisExt[http.DetectContentType(head)]
	if !ok {
		writeInvalid(w, invalid("Berkas itu bukan gambar PNG, JPG, atau WEBP."))
		return
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}

	base := s.qrisBase()
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		log.Printf("gagal membuat folder gambar QRIS: %v", err)
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}

	target := base + ext
	tmp := target + "." + randHex(4) + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		log.Printf("gagal menulis gambar QRIS: %v", err)
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	if _, err := io.Copy(out, io.LimitReader(file, maxQRISBytes)); err != nil {
		out.Close()
		os.Remove(tmp)
		log.Printf("gagal menyalin gambar QRIS: %v", err)
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	out.Close()
	if err := os.Chmod(tmp, 0o644); err != nil {
		log.Printf("gagal mengatur izin gambar QRIS: %v", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		log.Printf("gagal memasang gambar QRIS: %v", err)
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}

	// Buang versi lama dengan ekstensi lain supaya tidak ada dua gambar.
	for otherExt := range qrisExt {
		other := base + qrisExt[otherExt]
		if other != target {
			os.Remove(other)
		}
	}
	os.Remove(base + ".jpeg")

	name := "QRIS"
	if header != nil && strings.TrimSpace(header.Filename) != "" {
		name = rapikanNamaBerkas(filepath.Base(header.Filename))
	}
	_ = s.store.LogEvent("Gambar QRIS diperbarui (" + name + ").")

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "image_url": s.qrisURL()})
}

// handleQRISDelete menghapus gambar QRIS, misalnya kalau salah unggah.
func (s *Server) handleQRISDelete(w http.ResponseWriter, r *http.Request) {
	p := s.qrisFile()
	if p == "" {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	if err := os.Remove(p); err != nil {
		log.Printf("gagal menghapus gambar QRIS: %v", err)
		writeErr(w, http.StatusInternalServerError, "store_error")
		return
	}
	_ = s.store.LogEvent("Gambar QRIS dihapus.")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// reNamaBerkas dipakai untuk merapikan nama berkas pada catatan aktivitas.
var reNamaBerkas = regexp.MustCompile(`[^\w.\- ]`)

func rapikanNamaBerkas(nama string) string {
	nama = reNamaBerkas.ReplaceAllString(nama, "")
	if len(nama) > 60 {
		nama = nama[:60]
	}
	return strings.TrimSpace(nama)
}
