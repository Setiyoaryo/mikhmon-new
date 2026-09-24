# Custom Templates Per-Customer

Folder ini khusus untuk menyimpan kodingan kustom (HTML/CSS/JS/PHP) per customer.

## Struktur Folder

Cukup buat folder dengan nama yang **sama persis dengan subdomain customer**:

```text
custom-templates/
├── _contoh/                      # Contoh acuan template
│   ├── login.php                 # Custom login page panel Mikhmon
│   ├── brand.txt                 # Nama merek di panel & login
│   ├── logo.png                  # Logo custom panel & login
│   └── hotspot/                  # Halaman captive portal WiFi MikroTik (HP user)
│       ├── login.html
│       └── style.css
│
├── warkop-berkah/                # Otomatis aktif untuk warkop-berkah.nocify.id
│   ├── brand.txt                 # Isinya: "Warkop Berkah WiFi"
│   ├── logo.png                  # Logo warkop
│   └── hotspot/                  # Login page WiFi yang dilihat pelanggan warkop
│       └── login.html
│
└── taufiq/                       # Otomatis aktif untuk taufiq.nocify.id
```

## Jenis Kustomisasi yang Didukung

1. **Custom Login Panel Mikhmon (`login.php`)**
   - Jika file `custom-templates/<customer>/login.php` ada, halaman login panel Mikhmon (`https://<customer>.nocify.id/admin.php?id=login`) otomatis menggunakan file tersebut alih-alih login bawaan.
   - Form cukup mengirim `POST` dengan input `user` dan `pass` (lihat contoh di `_contoh/login.php`).

2. **Custom Nama Brand & Logo (`brand.txt` & `logo.png`)**
   - Jika tidak butuh custom login page full HTML, cukup taruh `brand.txt` atau `logo.png`.
   - Login page bawaan otomatis menampilkan logo dan nama merek tersebut.

3. **Custom MikroTik Captive Portal (`hotspot/`)**
   - File di dalam `custom-templates/<customer>/hotspot/` (seperti `login.html`, CSS, gambar) langsung bisa diakses via web di:
     `https://<customer>.nocify.id/hotspot-login/`
   - MikroTik customer cukup diarahkan ke URL tersebut.

## Cara Deploy / Sinkronisasi

### Opsi A: Otomatis via CI/CD (Rekomendasi)
Cukup commit & push ke branch `feat/portal-mockup` (atau `main`):
```bash
git add custom-templates/
git commit -m "feat: custom captive portal warkop-berkah"
git push
```
GitHub Actions otomatis menguji dan men-deploy template ke server.

### Opsi B: Sync Instan 1 Detik (CLI Tool)
Saat live-coding / styling bareng klien dan ingin preview langsung di staging tanpa git commit:
```bash
./tools/sync-template.sh <customer> staging
# Contoh:
./tools/sync-template.sh warkop-berkah staging
```
Template langsung ter-upload dan aktif dalam 1 detik.
