### MIKHMON V3

#### Download update.zip
[update.zip](https://raw.githubusercontent.com/laksa19/laksa19.github.io/master/download/update.zip){:target="_blank"}

### Changelog

#### Voucher batches up to 5000, and what happens to the ones nobody uses

Generating and clearing vouchers is now sized for real batches, measured
against a live RouterOS 6.49 CHR (single core):

| Action | 1000 vouchers | 5000 vouchers |
|---|---|---|
| Generate (form -> nginx -> PHP -> Go -> router) | ~0.45 s | **~2.3 s** |
| Delete by comment | ~0.5 s | **~9 s** |

Worker connections default to 16. More is not better: on a single core router,
32 connections measured *slower* than 16 for both add and delete, so the
default is the fast one.

What changed:
  - the Generate form accepts up to 5000 (was 500), with the PHP and nginx
    timeouts raised to match.
  - deletes go out as one parallel batch instead of one round trip per user.
    "By Comment" and "Expired Users" used to walk the list one user at a time;
    deleting a user through the user list used to cost six round trips *per
    user* because it looked up that user's script and scheduler separately.
  - attributes are omitted rather than sent empty, because RouterOS rejects an
    empty limit-uptime outright and that failed the entire batch.

#### Where unused vouchers go

Vouchers are MikroTik hotspot users, so they are inventory: left alone they
fill the user table, and every lookup gets slower as it grows. There are three
states and each has its own tool.

1. **Never used** (`uptime` still `0s`). Dead inventory once the selling period
   is over. Two ways to clear them, both of which only ever touch unused
   vouchers - a voucher that was logged in with keeps its uptime and is never
   selected:
   - *By Comment* (pick a batch in the user list) - clears one batch.
   - *Unused > 30d* (new button in the user list) - clears every batch whose
     comment is older than 30 days. Mikhmon already writes the generation date
     into each batch comment (`vc-735-09.22.26-name`), so age needs no extra
     bookkeeping. Comments without a date are skipped, and the retention window
     is a URL parameter (`&days=90`) if 30 is not what you want.
2. **Used and still valid.** Leave them; the user profile's validity handles it.
3. **Used and expired.** The profile scheduler rewrites their `limit-uptime` to
   `1s`, and *Expired Users* clears them.

Suggested routine: keep unused vouchers for your selling window (30 days is the
default here), then run *Unused > 30d* once a month. Do not delete used-but-
valid vouchers just to tidy up - that cuts off customers who paid.


The user interface is still the original Mikhmon v3 PHP application — same HTML,
same CSS themes, same JavaScript, same URLs. What changed is where the RouterOS
traffic happens.

1. **`mikhmon-api`, a Go backend service** (`cmd/mikhmon-api`):
   - Owns a pool of authenticated RouterOS API connections and reuses them
     across requests. The PHP version dialled and logged in once per request.
   - Creates hotspot users with a bounded worker pool, so a whole voucher batch
     is pushed in parallel instead of one sequential round-trip per user.
   - Speaks the RouterOS wire protocol directly (post-6.43 and pre-6.43 login),
     so no PHP extension or external client is involved.
2. **`lib/routeros_api.class.php` is now a bridge**: it keeps the exact public
   surface of the original `RouterosAPI` class (`connect`, `comm`, `write`,
   `read`, `parseResponse`, `debug`, plus the `encrypt`/`decrypt`/`rand*`
   helpers) and forwards every call to the Go service over HTTP. The service
   returns the same raw sentences a socket read would have produced, so
   `parseResponse()` and every caller behave exactly as before. No other PHP
   file had to be touched to make the switch.
3. **`hotspot/generateuser.php`** — only the two `for` loops that added users one
   at a time were replaced with one call to `mikhmon_bulk_add_hotspot_users()`.
   The credential generation, the form, and the printed output are untouched.
4. **Docker / Traefik**: `docker-compose.vps.yml` runs the PHP frontend
   (nginx + php-fpm in one container) and the Go service (internal network
   only, no published port) behind an existing Traefik instance.
5. **`cmd/mockrouteros`** — a fake RouterOS API endpoint for testing without a
   real MikroTik device. See `docker/README.md`.

#### Update 06-30 2021 V3.20
1. Perbaikan typo script profile ```on-login```.
	- Silakan update user profile dari Mikhmon, dengan cara membuka tiap user profile, kemudian klik Save.

#### Update 24-01 2021
1. Added docker-compose.yml for test-lab. added mikrotik routeros image.
	- git clone project
	- open project folder in terminal
	- run terminal command --> docker-compose up -d
	- go to localhost:8081. write ip address 192.168.88.1. write password 12345. apply configuration.
	- go to localhost:8080. user:mikhmon password:1234. add router. ip address 172.27.0.7, user:admin, password: 12345. write 'test' other inputs.last click save button
	- for stop --> docker-compose down
	
#### Update 09-08 2020 V3.19
1. Penambahan jumlah sisa voucher di "option comment" laman user list.

#### Update 04-07 2020
1. Added Dockerfile for test
	- git clone project
	- docker build --tag mikhmonv3 .
	- docker run --rm -i -t -p 8080:80 --name="mkhmn1" mikhmonv3
	- go to localhost:8080

#### Update 08-16 2019 V3.18
1. Penambahan harga jual. (Harga yang tampil di voucher)

	*update user profile isi harga jual(selling price) dan update juga template vouchernya, silakan download di [website](https://laksa19.github.io/?mikhmon/v3/voucher)
	
2. Untuk pengguna Termux, uninstall Mikhmon kemudian install lagi. 

#### Update 08-06 2019 V3.17
1. Perbaikan live report.
2. Perbaikan generate users.
3. Penambahan idle tileout (auto logout).
4. Penambahan ping IP Mikrotik di session settings.

#### Update 07-14 2019 V3.16
1. Penambahan address pool di add user profile dan edit user profile
2. Notif new update di admin settings

#### Update 07-02 2019 V3.15
1. Update RouterOS API for support v6.45.x

#### Update 05-09 2019 V3.14
1. Perbaikan time zone untuk print / quick print.
2. Penambahan input comment setelah comment user berubah menjadi tanggal expired.

	![314](https://raw.githubusercontent.com/laksa19/laksa19.github.io/master/img/3.14.gif)

#### Update 04-06 2019 V3.13 r7
1. Perbaikan add user profile (gagal membuat monitor profile di scheduler).
2. Perbaikan edit profile (remove monitor profile untuk expired mode none).
3. Penambahan indikator monitor profile di laman list user profile dan edit user profile (Green = Monitor Profile aktif, Orange = Monitor Profile tidak aktif).

	``` Monitor Profile adalah scheduler yang mengecek expired user ```

	![indicator](https://raw.githubusercontent.com/laksa19/laksa19.github.io/master/img/profile-indicator.png)

#### Update 04-02 2019 V3.13 r6
1. Perbaikan penghitungan tanggal dan jam monitor user profile. 
2. Perubahan global function ke local function. 

	Silakan diupdate kembali user profilenya. (buka user profile dari Mikhmon, simpan kembali masing-masing user profile).

	Setelah update user profile hapus semua environment (system -> scripts -> environment).

	![delenvironment](https://raw.githubusercontent.com/laksa19/laksa19.github.io/master/img/delenvironment.gif)

	Link Video [Update Profile v3.13 r6](https://drive.google.com/file/d/1ezFG0yxr3LOTgymH_ivUulF8MVevO2-V/view?usp=sharing)

#### Update 03-31 2019 V3.13 r5
1. Perbaikan user profile. (user expired dipergantian bulan). Silakan diupdate kembali user profilenya.
	[https://github.com/laksa19/mikhmonv3/issues/5](https://github.com/laksa19/mikhmonv3/issues/5)

#### Update 03-30 2019 V3.13 r4
1. Perbaikan edit user.
2. Penambahan nama profile di filter comment (user list).
3. Penambahan hapus expired user (klik expired pada kolom comment user list).
4. Perbaikan print laporan penjualan.

#### Update 03-27 2019 V3.13 r3
1. Perbaikan edit profile.
2. Perbaikan userlist (dobel comment di pilihan/filter user berdasarkan comment).
3. Penambahan changelog di laman About.

#### Update 03-22 2019 V3.13 r2
1. Perbaikan user profile, untuk data penjualan dobel (user 2 digit angka). Silakan diupdate kembali user profilenya.

#### Update 03-21 2019 V3.13 r1
1. Perbaikan user profile, untuk data penjualan tidak muncul di Mikhmon. Silakan diupdate kembali user profilenya.

#### Update 03-20 2019 V3.13
1. Perbaikan QR Code. Tidak lagi menggunakan Google chart API.
2. Perubahan variable QR Code menjadi <?= $qrcode ?> tanpa tag ```<img>```. 
	  
   ! Perlu penyesuaian untuk template hotspot, ubah 
  ```<img src="<?= $qrcode ?>" >``` menjadi ```<?= $qrcode ?>``` tanpa tag ```<img>```. Bagi yang menggunakan template default bisa reset template default untuk menyesuaikan QR Code.
	  
   Untuk template voucher yang lain bisa menyesuaikan ukuran QR Code dapat menambahkan style sebagai berikut.
   
```html
<style>
  .qrcode{
  height:80px;
  width:80px;
  }
</style>
```

![newqr](https://raw.githubusercontent.com/laksa19/laksa19.github.io/master/img/newqr.gif)
   
3. Penghapusan Grace period. 
4. Pehapusan info start dan end user.
5. Perubahan mode expired. 
	
	Mode baru ini tidak lagi menggunakan scheduler per user. Sebagai gantinya informasi tanggal expired akan dipindahkan ke comment user setelah login. Silakan update user profile agar dapat menggunakan mode expired yang baru. Pengecekan expired user yang login sebelum user profile diupdate atau yang masih menggunakan mode expired versi 3.12, bisa melalui scheduler di Mikhmon.

    ! Untuk yang menggunakan expired mode dengan record jangan update user profile yang sudah ada, sampai user dengan profile tersebut sudah habis. Sebaiknya buat user profile baru dan generate user baru dengan user profile tersebut. Apa yang terjadi jika diupdate? Report penjualan akan menjadi bertambah untuk masing-user yang sudah login. Tapi kalau tidak ada masalah dengan data penjualan yang double, silakan update user profilenya.

    ! User yang login sebelum user profile diupdate akan tetap menggunakan sistem atau mode expired yang lama.
    
    ! Jangan hapus atau mengganti comment user jika sudah menggunakan format tanggal sebagai berikut :
 		
	```mar/20/2019 16:05:11```.

6. Cek status voucher tidak bisa untuk user yang masih menggunakan profile dengan mode expired versi 3.12.

#### Update 03-12 2019 V3.12 r1
1. Perbaikan user profile. Meminimalisir user terhapus sesaat setelah login. !Silakan update user profile dari Mikhmon.

#### Update 03-08 2019 V3.12
1. Perbaikan remove session.
2. Penambahan print untuk report
3. Penambahan filter berdasarkan comment dan range tanggal. (Mikhmon Online).

#### Update 02-14 2019 V3.11
1. Perbaikan dashboard blank.
2. Penggantian Print Bluetooth dengan Quick Printer
3. Penambahan Quick Print. Panduan, https://youtu.be/KGAsHU0qOBA

#### Update 02-06 2019 V3.10
1. Perbaikan delete logo.
2. Penambahan pilihan bahasa.
3. Dukungan untuk print voucher dari Android. Telah diuji untuk Zjiang Printer Thermal Bluetooth - ZJ-5802.
Panduan, https://laksa19.github.io/printBT.html

#### Update 02-01 2019 V3.9 r3
1. Perbaaikan cek empty session laman admin
2. Perbaikan resume report, untuk menampilkan resume bulan sebelumnya.

#### Update 01-29 2019 V3.9 r2
1. Perbaaikan load time laman dashboard.
2. Perbaikan laman uploaad logo.

#### Update 01-29 2019 V3.9 r1
1. Perbaikan template voucher editor.
2. Penambahan short tabel.
3. Perbaikan reset hotspot user.

#### Update 01-27-2019 V3.9
1. Perbaikan CSS, penambahan tema Blue dan Green.
2. Cek Koneksi sebelum masuk dashboard dan berganti session.
3. Penambahan Indikator session Mikhmon yang aktif.
4. Penambahan fitur Resume Report.

#### Update 01-22-2019 V3.8
1. Perbaikan Theme.
2. Traffic dashboard dengan Highchart.
3. Penambahan fitur Traffic Monitor.

#### Update 01-17-2019 V3.7
1. Penambahan Light Theme.
2. Pennambahan menu penngganttian tema di navbar.

#### Update 12-21-2018 V3.6 r1
1. Penambahan Live Report

#### Update 12-1-2018 V3.6
1. Penambahan progrss bar.
2. Enable price use decimal (.).
3. Filter report by prefix.
4. Export user to script.
5. Export user to csv.
6. Penggantian kolom print menjadi tombol dan penambahan pilihan comment di user list.
7. Perubahan cara print voucher dari user list.
6. Beautify template editor dan penambahan tombol view voucher.

#### Update 11-9-2018 V3.5
1. Penambahan chart traffic. Sesuaikan Max Rx dan Tx di Settings.
2. Penambahan pilihan filter di Report dan User Log. 

#### Update 10-30-2018 V3.4
1. Penambahan cek spasi di nama user profile.
2. Penambahan user profile dan comment di Report. Yang perlu dilakukan adalah update user profile dari Mikhmon, buka user profile yang ingin diupdate kemudian klik Save. 
3. Penambahan filter berdasarkan server hotspot di Hotspot Active.

#### Update 10-24-2018 V3.3
1. Perubahan struktur menu.
2. Penambahan Hotspot Cookie dan System Scheduler.
3. Perubahan Generate User. Menghilangkan huruf l,L,q,Q,o,O serta angka 1 dan 0.
4. Perbaikan remove user.

#### Update 09-10-2018 V3.2
1. Penambahan kolom Time Left di Hotspot Active.
2. Penambahan Parent Queue di Add dan Edit User Profile (Bagaimana cara penggunaannya? silakan pelajari Simple Queue Mikrotik).
3. Penyesuaian format Data Limit user menjadi Byte Binary ([base 2](https://www.gbmb.org/gigabytes)).
4. Reformat Uptime.
