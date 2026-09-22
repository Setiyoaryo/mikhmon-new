# NOCIFY Billing — portal langganan Mikhmon

Mockup halaman portal, dibangun dengan **Svelte 5 + Vite 8**. Ini belum
tersambung ke backend apa pun; datanya contoh, tombolnya bekerja secara lokal.

Tujuannya memperlihatkan bentuk dan alur sebelum backend-nya dikerjakan.

## Menjalankan

```bash
cd portal
npm install
npm run dev          # server pengembangan, buka alamat yang ditampilkan
# atau
npm run build        # hasilnya di portal/dist
```

Setelah `npm run build`, folder `dist/` bisa disajikan sebagai file statis:

```bash
cd portal/dist && python3 -m http.server 4173
```

Lalu buka:

| Alamat | Isi |
|---|---|
| `/#/pay/NOC-8F3A-2C71` | Halaman pelanggan (tautan yang dikirim NOCIFY) |
| `/#/admin` | Halaman admin NOCIFY |
| `/pratinjau.html` | Pratinjau dengan pemilih halaman dan lebar layar |

`pratinjau.html` gunanya untuk meninjau di panel yang sempit: isinya digambar
pada lebar sungguhan (1280px) lalu diperkecil, jadi layout desktop-nya benar-benar
ter-render, bukan sekadar dikecilkan. Halaman itu bukan bagian dari portal.

Hasil build: sekitar **26 kB gzip** seluruhnya (HTML + CSS + JS), plus Font Awesome
yang disalin ke `public/font-awesome/` supaya tidak bergantung pada CDN.

### Menjalankan backendnya

```bash
go build -o /tmp/mikhmon-portal ./cmd/mikhmon-portal

MIKHMON_PORTAL_ADMIN_PASSWORD=rahasia \
MIKHMON_PORTAL_WEB_DIR="$PWD/portal/dist" \
MIKHMON_PORTAL_DB=/tmp/portal.db \
  /tmp/mikhmon-portal
```

Default-nya mendengarkan di `:8090`. Buka `http://127.0.0.1:8090/#/admin` dan
masuk memakai kata sandi itu. Pelanggan contoh dibuat otomatis; tautan
pembayarannya ada di `pay_url` pada `GET /api/v1/admin/overview`.

Di produksi keduanya dibangun jadi satu image lewat `Dockerfile.portal`, dan
antarmuka Svelte-nya di-embed ke dalam binary Go — tidak perlu nginx terpisah.

## Kenapa portal terpisah

Apa pun yang berjalan di server pelanggan ada di tangan pelanggan: gambar QRIS,
daftar harga, dan file lisensi semuanya bisa mereka ganti. Lisensi offline dengan
kunci HMAC tidak menolong, karena panel tetap harus menyimpan kunci rahasianya
untuk bisa memeriksa kode — jadi pelanggan bisa membacanya.

Karena itu aturannya satu: **halaman pembayaran tidak boleh berada di server
pelanggan.** Semuanya pindah ke satu portal yang hanya NOCIFY yang punya.

```
   SERVER NOCIFY (satu-satunya yang dikontrol)          SERVER PELANGGAN
 ┌───────────────────────────────────────────┐        ┌──────────────────────┐
 │  portal.nocify.id      (Go + SQLite)      │        │  Mikhmon (PHP)       │
 │  ┌─────────────────────────────────────┐  │        │  UI tetap sama       │
 │  │ /admin   pelanggan, paket, approve, │  │        │                      │
 │  │          tangguhkan                 │  │        │  Langganan:          │
 │  ├─────────────────────────────────────┤  │        │   status read-only   │
 │  │ /pay/<token>  QRIS + harga          │◄─┼────────┼── tautan ke portal   │
 │  │          (hanya ada di sini)        │  │        │                      │
 │  └─────────────────────────────────────┘  │        │  ┌────────────────┐  │
 │   SQLite: customers/instances/payments    │        │  │ heartbeat 6 jam│  │
 └───────────────────────────────────────────┘        │  │ + cache offline│  │
              ▲                                       │  └────────────────┘  │
              └──── POST /api/v1/heartbeat ───────────┘                      │
                    {instance_id, token}                                    │
                    → {state, expires, pay_url}                             │
                                                        └──────────────────────┘
```

Panel pelanggan tidak lagi menyimpan lisensi atau menampilkan QRIS. Dia hanya
bertanya sekali tiap beberapa jam: *"saya masih aktif?"*, lalu menyimpan
jawabannya untuk pemakaian offline.

| | |
|---|---|
| QRIS tidak bisa diganti | Tidak pernah ada di server pelanggan |
| Menangguhkan pelanggan | Satu tombol di portal; panel tahu di heartbeat berikutnya |
| Aktivasi | Baris di portal, bukan tukar-menukar kode |
| Internet portal mati | Panel memakai jawaban terakhir + masa tenggang, jadi tidak ikut mati |

## Halaman

**`#/pay/<token>` — halaman pelanggan.** Satu tautan unik per pelanggan, tanpa
login. Isinya status langganan (paket, mulai, berakhir, sisa hari, progress),
peringatan kalau tinggal ≤ 7 hari, pemilih paket, kartu QRIS, nominal yang harus
dibayar persis, tombol **Saya sudah bayar**, dan tombol WhatsApp. Setelah menekan
tombol itu muncul nomor referensi dan status menunggu verifikasi.

**`#/admin` — halaman admin NOCIFY.** Ringkasan (pendapatan bulan ini, aktif,
akan berakhir, kedaluwarsa), daftar klaim pembayaran yang menunggu **Setujui /
Tolak**, daftar pelanggan dengan pencarian dan tombol **Tangguhkan / Aktifkan**,
serta catatan aktivitas.

Bart "Mockup" di bawah hanya ada di mockup, untuk berpindah halaman dan mencoba
tiga keadaan langganan (aktif / mau habis / habis).

## Bentuk datanya

SQLite, dibuat otomatis di `data/portal/portal.db` saat pertama dijalankan.

```sql
customers  id, name, institution, wa, pay_token, valid_from, valid_until,
           suspended, created_at
instances  id (12 hex), token, customer_id, router_name, version, last_seen
plans      code, label, months, price, note, sort, sale
payments   id, customer_id, plan_code, amount, status, ref, created_at, decided_at
events     catatan aktivitas (audit sederhana)
```

Endpoint yang sudah jalan (semuanya di bawah `/api/v1`):

```
POST /heartbeat                          panel → portal
     {instance_id, token, version}
     → {state, plan, expires_at, days_left, pay_url, message, server_time}
     401 bad_token / 404 unknown_instance

GET  /pay/{token}                        pelanggan
     → {customer, instance, subscription, plans, qris, wa_number, pending_claim}
POST /pay/{token}/claim                  {plan_code} → klaim baru
     409 already_pending (membawa klaim yang sedang menunggu)

POST /admin/login      {password} → 204 + cookie
POST /admin/logout     → 204
GET  /admin/me         → {user}
GET  /admin/overview   → {stats, claims, customers, activity}
POST /admin/claims/{id}/approve        → memperpanjang langganan
POST /admin/claims/{id}/reject
POST /admin/customers/{id}/suspend     → panel jadi expired
POST /admin/customers/{id}/activate

GET  /admin/plans      → daftar paket yang dijual
GET  /admin/instances  → daftar pemasangan panel
POST /admin/instances  → buat pemasangan baru; token-nya ditampilkan sekali
POST /admin/customers  → tambah pelanggan
                         {name, institution, wa, session_name, instance_id, plan_code}
                         → {customer} beserta pay_url
PATCH  /admin/customers/{id}        → ubah nama, usaha, wa, subdomain, panel
POST   /admin/customers/{id}/extend → {months} → {expires_at}
DELETE /admin/customers/{id}

`session_name` harus sama dengan nama sesi router di panel, karena subdomain
itulah yang menentukan sesi mana yang dipakai (`include/tenant.php`). Masukan
yang ditolak dibalas `400 {"error":"invalid","message":"..."}` dengan kalimat
siap tampil, jadi antarmuka tinggal menampilkan `message`-nya.
```

Perpanjangan ditambahkan dari tanggal berakhir yang masih tersisa, bukan dari
hari ini, jadi pelanggan yang bayar lebih awal tidak kehilangan sisa waktunya.

## Pembayaran

QRIS-nya **statis** (GoPay Merchant), dan QRIS statis tidak punya notifikasi
otomatis. Jadi pembayaran selalu dikonfirmasi manual — bedanya sekarang di satu
layar portal, bukan berburu chat WhatsApp.

Kalau nanti mau otomatis, jalannya memakai QRIS dinamis dari penyedia PJP
(Midtrans, Xendit, Duitku, Pakasir): nominal dan kode unik dibuat per transaksi,
lalu webhook mengaktifkan sendiri. Tabel `payments` sudah menyediakan `status`
dan bisa ditambah kolom referensi eksternal tanpa mengubah yang lain.

## Yang belum

- Unggah gambar QRIS dari halaman admin (sekarang ditaruh manual sebagai
  `data/portal/qris.png`)
- Rate limit per Host di Traefik dan kuota konkurensi per tenant di
  `mikhmon-api`, supaya satu pelanggan tidak membanjiri yang lain
- Antarmuka untuk mengubah paket dan harganya (sekarang masih dari seeder)
- Pemindahan instalasi lama ke sesi per pelanggan di VPS produksi:
  `tools/mikhmon-migrate-sessions.php` lalu `tools/mikhmon-rekey.php`

## Batasan yang tetap ada

Pelanggan yang punya akses file tetap bisa menambal PHP-nya supaya tidak bertanya
ke portal. Yang benar-benar terlindungi adalah **uangnya** — dan itu bagian yang
penting.
