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

## Bentuk data yang diasumsikan

```sql
customers  id, name, institution, wa, created_at
instances  id (12 hex), token, customer_id, router_name, last_seen, version
plans      code, label, months, price
payments   id, customer_id, plan_code, amount, status, created_at, approved_at
ledger     catatan perubahan status (audit)
```

Rancangan endpoint:

```
POST /api/v1/heartbeat   panel → portal   {instance_id, token, version}
                                          → {state, expires, pay_url, message}
GET  /pay/<token>        pelanggan        status + QRIS + tombol klaim
POST /api/v1/claim       pelanggan        "saya sudah bayar"
GET  /api/v1/admin/...   admin            kelola pelanggan & pembayaran
```

## Pembayaran

QRIS-nya **statis** (GoPay Merchant), dan QRIS statis tidak punya notifikasi
otomatis. Jadi pembayaran selalu dikonfirmasi manual — bedanya sekarang di satu
layar portal, bukan berburu chat WhatsApp.

Kalau nanti mau otomatis, jalannya memakai QRIS dinamis dari penyedia PJP
(Midtrans, Xendit, Duitku, Pakasir): nominal dan kode unik dibuat per transaksi,
lalu webhook mengaktifkan sendiri. Tabel `payments` sudah menyediakan `status`
dan bisa ditambah kolom referensi eksternal tanpa mengubah yang lain.

## Yang belum

- Backend Go + SQLite (`cmd/mikhmon-portal`)
- Login admin
- Panel pelanggan: klien heartbeat dan halaman Langganan read-only
- Menghapus `tools/mikhmon-keygen.php` dan lisensi HMAC yang lama

## Batasan yang tetap ada

Pelanggan yang punya akses file tetap bisa menambal PHP-nya supaya tidak bertanya
ke portal. Yang benar-benar terlindungi adalah **uangnya** — dan itu bagian yang
penting.
