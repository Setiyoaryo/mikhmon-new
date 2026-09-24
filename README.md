# MIKHMON Cloud Multi-Tenant & Billing Portal (NOCIFY Edition)

> **Mikhmon V3 Modernized** — Fork modern dari MikroTik Hotspot Manager (Mikhmon v3) dengan backend Go berkecepatan tinggi, sistem sewa multi-tenant otomatis, billing portal pusat terintegrasi QRIS, serta dukungan custom login page & captive portal per-pelanggan.

---

## Penghargaan & Kredit (Credits)

- **Original Author**: [Laksamadi Guko](https://github.com/laksa19) — Pembuat asli [Mikhmon v3](https://github.com/laksa19/mikhmonv3). Tampilan antarmuka, alur kerja hotspot, dan logika inti voucher tetap dipertahankan 100% kompatibel dan identik.
- **NOCIFY Edition**: [Setiyo Aryo Winata](https://github.com/Setiyoaryo) / [NOCIFY](https://nocify.id) — Arsitektur backend Go berkinerja tinggi, central billing portal, multi-tenancy isolation, CI/CD otomatis, dan modular custom template engine.

Proyek ini dirilis di bawah lisensi **GNU General Public License v2.0 (GPL-2.0)**.

---

## Ikhtisar Arsitektur

Sistem terbagi menjadi 3 komponen utama yang berjalan di dalam Docker:

```text
               Internet (HTTPS)
                      │
                      ▼
            Traefik Reverse Proxy (Auto SSL Let's Encrypt)
           ┌──────────┴───────────────┐
           │                          │
           ▼                          ▼
   control.nocify.id          <tenant>.nocify.id
 (Central Billing Portal)     (Panel Mikhmon Multi-Tenant)
   [Go + SQLite + Svelte]       [PHP 7.4 + Nginx]
                                      │ (Internal HTTP)
                                      ▼
                                mikhmon-api
                           [Go Microservice Daemon]
                                      │ (RouterOS API Socket)
                                      ▼
                             Router MikroTik Fisik / CHR
```

### 1. Panel Mikhmon (`<nama>.nocify.id`)
- **Frontend & UI**: Tetap PHP asli v3 tanpa diubah menjadi template Go. Semua form, navigasi, kalkulator waktu, dan tema CSS byte-identical dengan Mikhmon v3 original.
- **Isolasi Subdomain**: Subdomain URL langsung menentukan sesi router (`taufiq.nocify.id` otomatis memuat router `taufiq`). Pemilih sesi manual dimatikan untuk keamanan.
- **Performa 19.000+ User (`hscache`)**: Menggunakan memory-caching sementara. Daftar 19.396 user yang awalnya memakan 29 detik per refresh dipangkas menjadi **0.04 detik** (640× lebih cepat).
- **Batch Pricing & Voucher Printing**: Harga voucher tercatat per-batch di luar git (`data/voucher/harga-batch.php`). Setelah generate, user langsung diarahkan ke halaman hasil dengan tombol cetak batch instan.

### 2. RouterOS Microservice API (`mikhmon-api`)
- Dibangun dengan **Go (Golang)**. Berjalan internal, tidak dibuka ke publik.
- **Connection Pooling**: Mempertahankan pool koneksi socket terotentikasi ke router. Tidak ada overhead login berulang per-klik HTTP.
- **Parallel Worker Concurrency**: Generate dan delete voucher diproses paralel dengan worker pool (optimal di concurrency 16).
  - Generate 1.000 voucher: **~0.45 detik**
  - Generate 5.000 voucher: **~2.27 detik**
  - Delete 1.000 voucher: **~0.5 detik**

### 3. Central Billing & Subscription Portal (`control.nocify.id`)
- Binary Go tunggal tanpa CGO dengan embedded frontend **Svelte 5 + Vite**.
- Basis data **SQLite** (`data/portal/portal.db`) dengan migrasi skema otomatis.
- **Fitur Utama**:
  - Manajemen pelanggan sewa (Tambah, Perpanjang, Tangguhkan, Hapus).
  - Manajemen paket harga dinamis (bisa diubah langsung dari browser tanpa sentuh kode).
  - Sistem verifikasi pembayaran berbasis QRIS statis (GoPay/BCA/ShopeePay/dll) dengan unggah gambar QRIS langsung dari dashboard.
  - Heartbeat client: Panel menanyakan lisensi ke portal secara berkala. Panel **tidak pernah mengunci diri sendiri** jika portal mengalami gangguan sesaat (grace period 7 hari).

---

## Kustomisasi Per-Pelanggan (Custom Templates)

Setiap pelanggan dapat memiliki halaman login panel khusus, logo kustom, dan captive portal WiFi MikroTik sendiri.

### Struktur Folder `custom-templates/`
Cukup buat folder sesuai **nama subdomain customer**:

```text
custom-templates/
├── _contoh/                      # Acuan template bawaan
│   ├── login.php                 # Custom login panel Mikhmon
│   ├── brand.txt                 # Nama merek hotspot/usaha
│   ├── logo.png                  # Logo custom (muncul di login & panel)
│   └── hotspot/                  # Halaman Captive Portal WiFi MikroTik
│       ├── login.html
│       └── style.css
│
├── taufiq/                       # Aktif untuk taufiq.nocify.id
└── warkop-berkah/                # Aktif untuk warkop-berkah.nocify.id
```

### Cara Kerja & URL
| Kustomisasi | File | URL Akses |
|---|---|---|
| **Login Panel Admin** | `custom-templates/<customer>/login.php` | `https://<customer>.nocify.id/admin.php?id=login` |
| **Brand & Logo** | `brand.txt` / `logo.png` | Otomatis mengganti logo & teks di panel login default |
| **Captive Portal WiFi** | `custom-templates/<customer>/hotspot/` | `https://<customer>.nocify.id/hotspot-login/` |

> Jika customer tidak memiliki folder kustomisasi, panel berjalan dalam **mode default Mikhmon 100%**.

---

## Cara Sinkronisasi & CI/CD Deployment

### 1. Live Tweak Instan 1 Detik (CLI Tool)
Saat sedang live coding bersama klien dan butuh preview langsung tanpa bolak-balik git commit:

```bash
# Sinkronisasi ke Staging (staging.nocify.id)
./tools/sync-template.sh <subdomain> staging

# Sinkronisasi langsung ke Production (nocify.id)
./tools/sync-template.sh <subdomain> prod
```

### 2. Otomatis via GitHub Actions (CI/CD)
Setiap push ke remote repository akan memicu pipeline otomatis:
- **Push ke branch `feat/portal-mockup`**:
  - Validasi sintaks PHP (`php -l`).
  - Deploy otomatis via SSH ke server **Staging** (`/opt/mikhmon-staging`).
  - Aktif di `https://staging.nocify.id` dan `https://<customer>.staging.nocify.id`.
- **Push ke branch `main`**:
  - Validasi sintaks dan unit test.
  - Deploy otomatis via SSH ke server **Production** (`/opt/mikhmon-new`).
  - Aktif di `https://<customer>.nocify.id` dan `https://control.nocify.id`.

---

## Panduan Instalasi & Menjalankan Stack

### Kebutuhan Server
- Linux (Ubuntu 22.04 / Debian 12 direkomendasikan).
- Docker Engine & Docker Compose v2.
- Traefik Reverse Proxy pada docker network `proxy`.

### 1. Clone & Konfigurasi Lingkungan
```bash
git clone https://github.com/Setiyoaryo/mikhmon-new.git /opt/mikhmon-new
cd /opt/mikhmon-new

cp .env.example .env
nano .env
```

Pastikan variabel wajib diisi di `.env`:
```ini
PORTAL_ADMIN_USER=admin
PORTAL_ADMIN_PASSWORD=rahasia_admin_portal
PORTAL_BASE_URL=https://control.nocify.id
PORTAL_QRIS_MERCHANT="NOCIFY, SOFTWARE"
PORTAL_QRIS_NMID=ID1026599320839
PORTAL_WA=6285139495106
HSCACHE_TTL=300
```

### 2. Menjalankan Production Stack
```bash
docker compose -f docker-compose.vps.yml up -d --build
```

### 3. Menjalankan Staging Stack (Opsional)
Staging berjalan terisolasi di folder dan container terpisah, lengkap dengan mock router built-in:
```bash
git clone -b feat/portal-mockup https://github.com/Setiyoaryo/mikhmon-new.git /opt/mikhmon-staging
cd /opt/mikhmon-staging

docker compose -f docker-compose.staging.yml up -d --build
```

---

## Manajemen Operasional Harian

### Menambah Pelanggan Baru (3 Langkah)
1. **Buat sesi router**: Buka `https://panel.nocify.id` -> *Settings* -> *Add Router*. Isi nama sesi dengan **subdomain** yang diinginkan (huruf kecil, misal `budi`).
2. **Daftarkan di portal**: Buka `https://control.nocify.id/#/admin` -> *Tambah Pelanggan*. Masukkan nama, nomor WA, dan pilih sesi `budi`.
3. **Kirim info ke pelanggan**: Kirim alamat panel `https://budi.nocify.id` dan link aktivasi pembayaran.

### Mengubah Harga Langganan
Buka `https://control.nocify.id/#/admin` -> gulir ke kartu **Paket & Harga**. Klik nominal harga yang ingin diubah, ketik angka baru, lalu klik **Simpan**. Harga di halaman pembayaran pelanggan langsung terupdate tanpa restart container.

### Backup Data
Semua data penting tersimpan di folder `data/` yang tidak terlacak git:
- `data/portal/portal.db` (Database langganan & pelanggan SQLite).
- `data/templates/` (Template voucher yang disunting via editor).
- `data/voucher/harga-batch.php` (Catatan riwayat harga per-batch).
- `include/sessions/` & `include/tenantkey.php` (Kredensial router terenkripsi).

Cadangkan berkas tersebut secara berkala:
```bash
tar -czvf backup-mikhmon-$(date +%F).tar.gz /opt/mikhmon-new/data/ /opt/mikhmon-new/include/sessions/ /opt/mikhmon-new/include/tenantkey.php
```
