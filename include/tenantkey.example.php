<?php
/*
 *  Mikhmon - kunci enkripsi per sesi router (contoh).
 *
 *  Salin berkas ini menjadi include/tenantkey.php lalu isi satu kunci untuk
 *  tiap sesi. Kuncinya membuat password router hanya bisa dibaca oleh
 *  instalasi ini: berkas sesi yang bocor tidak bisa didekripsi di tempat lain,
 *  dan kunci satu pelanggan tidak bisa membuka password pelanggan lain.
 *
 *  Cara mengisi:
 *    1. Buat kunci 64 karakter hex, misalnya:
 *         php -r "echo bin2hex(random_bytes(32)), PHP_EOL;"
 *    2. Atau biarkan tool yang mengurus semuanya (kunci baru + enkripsi ulang
 *       password lama ke format v2):
 *         php tools/mikhmon-rekey.php --all
 *
 *  Contoh isi:
 *      $mikhmon_session_keys = array(
 *        'taufiq' => '3f1c... 64 karakter hex ...',
 *        'budi'   => 'a09e... 64 karakter hex ...',
 *      );
 *
 *  Selama include/tenantkey.php belum ada, password baru masih memakai
 *  algoritma lama (kunci 128) supaya instalasi lama tidak rusak. Setelah
 *  kunci dibuat, jalankan tools/mikhmon-rekey.php supaya password lama ikut
 *  dipindahkan ke format v2.
 *
 *  include/tenantkey.php TIDAK ikut di-commit (lihat .gitignore).
 */

// Penjaga akses langsung: berkas ini dieksekusi, jadi tidak ada isi yang bocor.
if (isset($_SERVER['REQUEST_URI']) && substr($_SERVER['REQUEST_URI'], -14) == 'tenantkey.php') {
  header('Location:./');
}

$mikhmon_session_keys = array(
  // 'nama-sesi' => 'kunci-hex-64-karakter',
);
