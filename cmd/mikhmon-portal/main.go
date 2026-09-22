// Command mikhmon-portal menjalankan portal langganan NOCIFY Billing.
//
// Portal ini melayani tiga hal:
//   - heartbeat untuk panel Mikhmon pelanggan (/api/v1/heartbeat)
//   - halaman pembayaran pelanggan (/#/pay/<token>)
//   - halaman admin NOCIFY (/#/admin)
//
// Pengaturan seluruhnya lewat variabel lingkungan, lihat docker-compose.vps.yml.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Setiyoaryo/mikhmon-new/internal/portal"
)

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// loadOrCreateSecret membaca kunci penanda tangan cookie dari berkas, dan
// membuatnya sekali kalau belum ada. Disimpan di berkas supaya sesi admin tidak
// ikut hilang setiap container dijalankan ulang.
func loadOrCreateSecret(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s, nil
		}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	encoded := hex.EncodeToString(secret)
	if err := os.WriteFile(path, []byte(encoded+"\n"), 0o600); err != nil {
		return "", err
	}
	log.Printf("kunci sesi baru dibuat di %s", path)
	return encoded, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	dbPath := env("MIKHMON_PORTAL_DB", "./data/portal.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("tidak bisa membuat folder basis data: %v", err)
	}

	password := strings.TrimSpace(os.Getenv("MIKHMON_PORTAL_ADMIN_PASSWORD"))
	if password == "" {
		log.Fatal("MIKHMON_PORTAL_ADMIN_PASSWORD belum diisi. " +
			"Set kata sandi admin portal dulu, jangan dibiarkan kosong.")
	}

	secret := strings.TrimSpace(os.Getenv("MIKHMON_PORTAL_SECRET"))
	if secret == "" {
		var err error
		secret, err = loadOrCreateSecret(filepath.Join(filepath.Dir(dbPath), "portal.secret"))
		if err != nil {
			log.Fatalf("tidak bisa menyiapkan kunci sesi: %v", err)
		}
	}

	store, err := portal.OpenStore(dbPath)
	if err != nil {
		log.Fatalf("tidak bisa membuka basis data %s: %v", dbPath, err)
	}
	defer store.Close()

	if err := store.Seed(); err != nil {
		log.Fatalf("gagal menyiapkan data awal: %v", err)
	}

	cfg := portal.Config{
		Addr:         env("MIKHMON_PORTAL_ADDR", ":8090"),
		DBPath:       dbPath,
		WebDir:       env("MIKHMON_PORTAL_WEB_DIR", ""),
		BaseURL:      env("MIKHMON_PORTAL_BASE_URL", "http://localhost:8090"),
		QRISImage:    env("MIKHMON_PORTAL_QRIS_IMAGE", "./data/qris.png"),
		QRISMerchant: env("MIKHMON_PORTAL_QRIS_MERCHANT", "NOCIFY, SOFTWARE"),
		QRISNMID:     env("MIKHMON_PORTAL_QRIS_NMID", "ID1026599320839"),
		WANumber:     env("MIKHMON_PORTAL_WA", "6285139495106"),
		Enroll:       envBool("MIKHMON_PORTAL_ENROLL", true),
		Auth: portal.NewAuth(
			env("MIKHMON_PORTAL_ADMIN_USER", "admin"),
			password,
			secret,
			envBool("MIKHMON_PORTAL_SECURE_COOKIE", true),
			12*time.Hour,
		),
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           portal.NewServer(cfg, store).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("portal jalan di %s (basis data %s, alamat publik %s)", cfg.Addr, dbPath, cfg.BaseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server berhenti: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("gagal berhenti dengan rapi: %v", err)
	}
	log.Println("portal berhenti")
}
