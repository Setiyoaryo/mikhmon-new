# Panduan pindah ke production

Untuk VPS yang sudah menjalankan Mikhmon (`/opt/mikhmon-new`) dan akan pindah
ke model langganan: panel tetap di tempatnya, ditambah portal di
`control.nocify.id`, dan setiap pelanggan dapat `<nama>.nocify.id`.

## Yang paling penting: pelanggan tidak boleh hilang

Ada tiga tempat data pelanggan, dan hanya satu yang disentuh oleh pemindahan
ini:

| Data | Di mana | Disentuh deploy? |
|---|---|---|
| **Kredensial router** (IP, user, password) | `include/config.php` → dipindah ke `include/sessions/<nama>.php` | **Ya - ini yang harus dijaga** |
| Akun admin panel | `include/config.php` (tetap di situ) | Tidak |
| User hotspot, voucher, profil | Di dalam router MikroTik | Tidak pernah |
| Catatan langganan | `data/portal/portal.db` (baru, awalnya kosong) | Baru dibuat |

Jadi "pelanggan hilang" hanya bisa terjadi kalau `include/config.php` rusak
saat dipindah. Alat pemindahnya sudah diuji dengan konfigurasi seperti
production (beberapa sesi, 11 bidang per sesi): **tidak ada satu bidang pun
yang hilang**, dan berkas aslinya selalu dicadangkan lebih dulu.

User hotspot dan voucher **tidak pernah disentuh** - semuanya ada di dalam
router, bukan di panel.

## Dua hal yang wajib dicadangkan

1. `include/` seluruhnya - berisi konfigurasi router dan (nanti) kunci
   enkripsi.
2. `include/tenantkey.php` setelah langkah 5 di bawah. **Berkas ini tidak bisa
   dibuat ulang.** Password router disimpan dengan kunci di dalamnya; kalau
   berkasnya hilang, semua password router harus diketik ulang. Masukkan ke
   cadangan rutin, dan jangan pernah ikut di-commit.

## Catatan penting soal bind mount

Direktori aplikasi di-bind-mount (`./:/var/www`), jadi **perubahan berkas PHP
langsung berlaku saat itu juga** - tidak ada tombol "deploy" yang bisa
dibatalkan. Karena itu urutan di bawah sengaja dibuat supaya setiap langkah
aman ditinggalkan di tengah jalan:

- Kode baru **masih bisa membaca `config.php` versi lama**, jadi setelah
  `git checkout` panel tetap jalan seperti sebelumnya.
- Pemindahan sesi membuat cadangan otomatis di `include/config.php.bak`.
- Enkripsi ulang membuat cadangan tiap berkas sesi (`.php.bak`).
- Nilai password lama tetap bisa dibaca setelah kunci baru dipakai.

## Urutan pengerjaan

### 0. Cadangan

```bash
cd /opt/mikhmon-new
mkdir -p ~/cadangan
cp include/config.php ~/cadangan/config.php.$(date +%Y%m%d-%H%M)
tar czf ~/cadangan/mikhmon.$(date +%Y%m%d-%H%M).tar.gz include/ data/ docker-compose.vps.yml .env 2>/dev/null
git rev-parse HEAD | tee ~/cadangan/head-sebelum.txt
docker compose -f docker-compose.vps.yml ps > ~/cadangan/container-sebelum.txt
```

Simpan `head-sebelum.txt` - itu titik balik kalau ada yang salah.

### 1. Tarik kode baru

```bash
git fetch origin
git checkout feat/portal-mockup      # atau nama branch setelah di-push
```

Panel langsung memakai kode baru. **Kode baru masih membaca `config.php` versi
lama**, jadi di titik ini belum ada yang berubah.

### 2. Periksa panel masih normal

Masuk ke `taufiq.nocify.id`, pastikan daftar router tampil, buka satu halaman
(Hotspot → User List), dan angka jumlah user muncul. Jangan lanjut kalau belum.

### 3. Pindahkan sesi ke berkas terpisah

```bash
php tools/mikhmon-migrate-sessions.php
```

Menghasilkan `include/sessions/<nama>.php` untuk setiap router, menulis ulang
`include/config.php` (hanya akun admin + pemuatnya), dan menyimpan salinan
asli di `include/config.php.bak`.

### 4. Periksa lagi

Halaman yang sama seperti langkah 2 harus tetap normal.

### 5. Terbitkan kunci dan enkripsi ulang password

```bash
php tools/mikhmon-rekey.php --all
cp include/tenantkey.php ~/cadangan/tenantkey.$(date +%Y%m%d-%H%M)
```

Setelah ini password router disimpan dengan kunci per pelanggan, bukan lagi
dengan kunci tetap yang bisa dibuka siapa pun.

### 6. Periksa password router masih terbaca

Buka **Settings → session → password MikroTik** harus menampilkan password
yang benar (bukan kosong), dan **Hotspot → User List** harus menampilkan data
dari router. Kalau password tampil kosong, kembalikan
`include/sessions/<nama>.php.bak` lalu periksa lagi.

### 7. Naikkan portal

Siapkan `.env` di `/opt/mikhmon-new`:

```
PORTAL_ADMIN_PASSWORD=<sandi kuat, jangan yang mudah ditebak>
PORTAL_BASE_URL=https://control.nocify.id
PORTAL_WA=6285139495106
```

| Variabel | Isi |
|---|---|
| `PORTAL_ADMIN_PASSWORD` | sandi masuk halaman admin portal. Wajib - portal menolak jalan tanpa ini |
| `PORTAL_BASE_URL` | **alamat publik portal**, lengkap dengan `https://`, tanpa garis miring di akhir. Inilah yang ditempel di depan tautan pembayaran pelanggan jadi `https://control.nocify.id/#/pay/NOC-...`, dan yang muncul di tombol WhatsApp. Jangan diisi `localhost` atau alamat dalam Docker |
| `PORTAL_WA` | nomor WhatsApp yang dihubungi pelanggan, format internasional tanpa `+` |

Lalu:

```bash
docker compose -f docker-compose.vps.yml up -d --build
docker compose -f docker-compose.vps.yml ps
docker logs -f mikhmon-portal
```

Build pertama lebih lama karena image Go naik ke 1.25.

### 8. Buat panel di portal

Buka `https://control.nocify.id/#/admin`, masuk, lalu **Panel terpasang →
Tambah panel**. Salin isi `include/instance.php` yang ditampilkan.

### 9. Pasang berkas instance di panel

```bash
cd /opt/mikhmon-new
cp include/instance.example.php include/instance.php
# tempel id, token, dan portal dari langkah 8
chmod 600 include/instance.php
```

### 10. Daftarkan pelanggan yang sudah ada

```bash
php tools/mikhmon-import-to-portal.php \
    --portal=https://control.nocify.id \
    --password=<sandi admin portal> \
    --instance=<id panel dari langkah 8> \
    --until=$(date -d "+30 days" +%Y-%m-%d) \
    --dry-run
```

Periksa hasil `--dry-run`, lalu jalankan tanpa `--dry-run`.

**Kenapa `--until`:** semua pelanggan yang diimpor langsung diberi masa
berlaku, supaya tidak ada panel yang terkunci pada hari pindah. Sesuaikan
tanggal masing-masing di halaman admin setelah itu.

Alat ini aman dijalankan berulang: pelanggan yang sudah ada dilaporkan "sudah
ada", bukan dibuat dobel.

**Perhatikan peringatan nama sesi.** Nama sesi harus huruf kecil semua supaya
cocok dengan subdomain. Kalau panel Anda memakai nama seperti `HOTSPOT`, alat
ini akan memperingatkan; ganti dulu namanya di **Settings** menjadi `hotspot`
(atau nama lain huruf kecil), baru jalankan lagi.

### 11. DNS dan sertifikat

Di pengelola DNS `nocify.id`, tambahkan satu record:

| Kolom | Isi |
|---|---|
| Type | `A` |
| Name / Host | `*` |
| Value / Points to | IP VPS |
| TTL | Automatic |

Itu satu record untuk semua: berlaku untuk `control.nocify.id`, untuk
`taufiq.nocify.id`, dan untuk subdomain pelanggan mana pun yang dibuat nanti.
Record `taufiq` yang sudah ada boleh dibiarkan - record khusus selalu menang
atas wildcard.

Yang perlu diperhatikan:

- Tanda `*` **tidak mencakup domain telanjang** `nocify.id` tanpa subdomain.
  Kalau alamat itu juga mau dipakai, tambahkan record `A` terpisah dengan
  Name `@`.
- **Kalau DNS-nya di Cloudflare, pilih "DNS only" (awan abu-abu), jangan
  "Proxied" (awan oranye).** Traefik menerbitkan sertifikat lewat HTTP-01 yang
  harus sampai ke VPS Anda; proxy Cloudflare memutus itu. Selain itu proxy
  wildcard baru tersedia di paket berbayar.
- Wildcard DNS butuh beberapa menit sampai berlaku. Periksa dengan:

```bash
dig +short uji.nocify.id
```

Traefik menerbitkan sertifikat per subdomain lewat HTTP-01 begitu subdomain
itu pertama kali dibuka. Let's Encrypt membatasi **50 sertifikat baru per
domain per minggu** - jangan buka puluhan subdomain dalam sehari.

### 12. Gambar QRIS

Buka **halaman admin portal → kartu Pembayaran QRIS → Unggah QRIS**, lalu
pilih berkas PNG/JPG-nya (maksimal 2 MB). Halaman pembayaran pelanggan
langsung memakainya.

Kalau lebih suka lewat baris perintah:

```bash
cp qris.png /opt/mikhmon-new/data/portal/qris.png
```

### 13. Periksa akhir

- `https://control.nocify.id/#/admin` - daftar pelanggan lengkap
- Buka tautan pembayaran salah satu pelanggan - status, QRIS, dan tombol
  klaim jalan
- `https://<nama>.nocify.id` - panel terbuka dan menampilkan data router
- Menu **Langganan** di panel - status sesuai, bukan "Belum Terhubung"
- Coba **Tangguhkan** satu pelanggan di portal, buka panelnya, tekan
  **Periksa sekarang** di menu Langganan - panel harus terkunci
- Aktifkan kembali

## Kalau ada yang salah

**Panel tidak bisa dibuka / daftar router kosong**
```bash
cd /opt/mikhmon-new
git checkout $(cat ~/cadangan/head-sebelum.txt)
cp ~/cadangan/config.php.<tanda-waktu> include/config.php
```
Kode lama membaca format lama, jadi ini mengembalikan keadaan semula.

**Password router tampil kosong setelah langkah 5**
```bash
cp include/sessions/<nama>.php.bak include/sessions/<nama>.php
```
Password lama masih bisa dibaca selama `include/tenantkey.php` ada.

**Portal tidak bisa dibuka**
```bash
docker compose -f docker-compose.vps.yml logs mikhmon-portal | tail -30
```
Kesalahan yang paling sering: `PORTAL_ADMIN_PASSWORD` belum diisi (portal
menolak jalan tanpa itu).

**Semuanya dihentikan**
```bash
docker compose -f docker-compose.vps.yml down
git checkout $(cat ~/cadangan/head-sebelum.txt)
cp ~/cadangan/config.php.<tanda-waktu> include/config.php
rm -f include/instance.php
docker compose -f docker-compose.vps.yml up -d --build mikhmon-php mikhmon-api
```

Menghentikan atau menghapus portal **tidak** mematikan panel: panel memakai
jawaban terakhir yang tersimpan, dan baru terkunci setelah 7 hari tanpa kabar.

## Yang perlu diingat setelah pindah

- `include/tenantkey.php` dan `data/portal/` masuk jadwal cadangan.
- Jangan pernah menghapus `include/tenantkey.php` selama masih ada password
  router yang tersimpan.
- Menambah pelanggan baru: buat sesinya di panel dengan nama yang **sama
  persis** dengan subdomain yang diinginkan (huruf kecil), lalu tambahkan
  pelanggannya di portal dengan subdomain yang sama.
