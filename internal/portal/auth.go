package portal

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// sessionCookie adalah nama cookie sesi admin.
const sessionCookie = "mikhmon_portal"

// Auth menangani login admin portal.
//
// Sengaja sederhana: satu akun admin, kata sandinya dari variabel lingkungan,
// dan sesinya cookie yang ditandatangani HMAC. Tidak ada tabel pengguna dan
// tidak ada pendaftaran - portal ini hanya dipakai NOCIFY.
type Auth struct {
	user   string
	pass   string
	secret []byte
	secure bool
	ttl    time.Duration
}

// NewAuth menyiapkan penjaga sesi admin.
func NewAuth(user, pass, secret string, secure bool, ttl time.Duration) *Auth {
	return &Auth{
		user:   user,
		pass:   pass,
		secret: []byte(secret),
		secure: secure,
		ttl:    ttl,
	}
}

// Authenticate memeriksa kata sandi. Perbandingannya waktu-tetap supaya
// panjang kata sandi tidak bisa ditebak dari selisih waktu balasan.
func (a *Auth) Authenticate(password string) bool {
	want := sha256.Sum256([]byte(a.pass))
	got := sha256.Sum256([]byte(password))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

// User mengembalikan nama pengguna admin.
func (a *Auth) User() string { return a.user }

func (a *Auth) sign(exp int64) string {
	payload := strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (a *Auth) verify(value string) bool {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	expected := a.sign(exp)
	return hmac.Equal([]byte(expected), []byte(value))
}

// SetCookie mengirim cookie sesi yang sudah ditandatangani.
func (a *Auth) SetCookie(w http.ResponseWriter) {
	value := a.sign(time.Now().Add(a.ttl).Unix())
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    base64.RawURLEncoding.EncodeToString([]byte(value)),
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(a.ttl.Seconds()),
	})
}

// ClearCookie menghapus cookie sesi.
func (a *Auth) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// LoggedIn melaporkan apakah permintaan ini membawa sesi admin yang sah.
func (a *Auth) LoggedIn(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return false
	}
	return a.verify(string(raw))
}
