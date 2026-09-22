# Mikhmon — Docker ops

Tiga layanan:

| Service         | Build file        | Isi                                                        | Paparan                                        |
| --------------- | ----------------- | ---------------------------------------------------------- | ---------------------------------------------- |
| `mikhmon-php`   | `Dockerfile.php`  | nginx + php-fpm (satu container), UI PHP asli              | Publik via Traefik, melayani `*.nocify.id`      |
| `mikhmon-api`   | `Dockerfile`      | Go microservice `mikhmon-api` di :8088                     | **Tanpa port publik** — hanya dari php-fpm      |
| `mikhmon-portal`| `Dockerfile.portal`| Portal langganan: Go + SQLite + antarmuka Svelte di-embed | Publik via Traefik, `control.nocify.id`         |

## Build

```bash
docker compose -f docker-compose.vps.yml build
```

## Run

```bash
docker compose -f docker-compose.vps.yml up -d --build
```

Butuh Docker network eksternal bernama `proxy` (Traefik) dan instance Traefik
dengan entrypoint `web`/`websecure` serta certresolver `letsencrypt`.

Direktori aplikasi di-bind-mount (`./:/var/www`), jadi `include/config.php`,
`include/sessions/`, voucher yang dihasilkan, tema/bahasa, dan logo yang
diunggah hidup di host dan tidak hilang saat container dibangun ulang. Basis
data portal ada di `./data/portal`.

## Logs

```bash
docker logs -f mikhmon-php      # nginx + php-fpm (supervisord menggabungkan keduanya)
docker logs -f mikhmon-api      # microservice Go
docker logs -f mikhmon-portal   # portal langganan
```

## Override MIKHMON_API_URL

`mikhmon-php` mendapat `MIKHMON_API_URL=http://mikhmon-api:8088` secara bawaan.
Untuk mengarahkannya ke tempat lain, ubah `environment:` di
`docker-compose.vps.yml` lalu buat ulang container:

```bash
docker compose -f docker-compose.vps.yml up -d --force-recreate mikhmon-php
```

`docker-compose.yml` adalah stack dev lokal yang lama (Traefik + nginx terpisah
+ RouterOS palsu) dan tidak berhubungan dengan produksi.

---

# Model deployment: hosted bersama

Panel di-host di VPS ini dan melayani semua subdomain `*.nocify.id` — satu
pelanggan, satu subdomain, satu sesi router. Portal langganan ada di
`control.nocify.id`.

## Kenapa satu VPS untuk banyak pelanggan

Angka terukur dari container yang berjalan:

| Komponen      | Memori diam |
| ------------- | ----------- |
| `mikhmon-php` | 11,6 MB     |
| `mikhmon-api` | 5,0 MB      |

Semua pelanggan berbagi satu container dan satu pool php-fpm, jadi menambah
pelanggan hampir tidak menambah memori — cukup satu subdomain dan satu berkas
sesi. Bukan 17 MB dikali jumlah pelanggan.

Dari sisi harga: paket termurah Rp 50.000/bulan, sedangkan VPS terkecil saja
Rp 50–80rb/bulan. Satu VPS per pelanggan langsung rugi sebelum dihitung waktu
mengurusnya. Satu VPS 2 vCPU / 4 GB (± Rp 150–250rb) menampung 20–30 panel.
Paket dedicated baru masuk akal kalau dijual **≥ Rp 300rb/bulan**.

Saran penskalaan: kalau mulai penuh, **tambah VPS kedua dan sebar tenant ke
situ**, jangan besarkan satu VPS. Dua VPS 2 vCPU/4 GB dengan 15 pelanggan
masing-masing jauh lebih tahan daripada satu VPS 8 vCPU/16 GB dengan 30
pelanggan — kalau satu mati, separuh pelanggan masih jalan.

## Batas antar pelanggan

**Sudah ada:**

- Subdomain menentukan sesi router (`include/tenant.php`). `?session=` tidak
  bisa dipakai menembus sesi pelanggan lain, dan pemilih sesi di sidebar
  disembunyikan. `control`, `www`, `api`, `portal` bukan milik pelanggan, dan
  `xnocify.id` tidak ikut tertangkap aturan wildcard.
- Kredensial router tiap pelanggan tidak lagi menumpuk di satu berkas.
- Password router disimpan dengan kunci per pelanggan.
- Traefik: panel prioritas 10, portal prioritas 100, supaya `control.nocify.id`
  tidak ikut masuk ke panel.

**Belum ada:**

- Rate limit per Host di Traefik.
- Kuota konkurensi per tenant di `mikhmon-api` (sekarang 16, global untuk semua
  router).

## Yang perlu diketahui soal isolasi

Berbagi satu VPS bukan benteng yang kuat antar pelanggan. Bentengnya adalah
berkas yang tidak dimuat: permintaan milik satu pelanggan tidak pernah membuka
kredensial pelanggan lain. Tapi kalau ada satu celah baca-berkas di aplikasi,
semua pelanggan di VPS yang sama bisa terpengaruh.

Untuk pelanggan yang butuh isolasi sungguhan, pindahkan ke VPS sendiri. Panel
dan portalnya sama persis — cukup pasang `include/instance.php` di VPS itu dan
buat instance baru di portal. Tidak ada kode yang perlu diubah.

## Menambah pelanggan baru

1. **DNS** (sekali saja): `*.nocify.id` A record ke VPS ini.
2. **Sertifikat**: Traefik menerbitkan sertifikat per subdomain lewat HTTP-01,
   jadi tidak perlu DNS challenge. Perlu diingat: Let's Encrypt membatasi 50
   sertifikat baru per domain per minggu — jangan onboarding puluhan pelanggan
   dalam sehari.
3. **Di panel**: buat sesinya lewat
   `admin.php?id=settings&session=new-<nama>`. Namanya harus sama dengan
   subdomain, misalnya `taufiq` untuk `taufiq.nocify.id`.
4. **Di portal**: buat pelanggan beserta sesinya, lalu kirim tautan pembayaran
   (`/#/pay/<pay_token>`) ke pelanggan itu.
