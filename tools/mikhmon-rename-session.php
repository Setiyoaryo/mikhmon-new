#!/usr/bin/env php
<?php
/*
 *  Mikhmon Nocify - ganti nama sesi router.
 *
 *  Copyright (C) 2024 NOCIFY.  GPLv2.
 *
 *  Dipakai kalau nama sesi tidak cocok dengan subdomain yang diinginkan.
 *  Subdomain selalu huruf kecil, jadi sesi bernama "HOTSPOT" tidak akan pernah
 *  dibuka lewat <nama>.nocify.id - namanya harus sama persis, misalnya
 *  "taufiq" kalau panelnya dibuka di taufiq.nocify.id.
 *
 *  Cara pakai:
 *
 *    php tools/mikhmon-rename-session.php HOTSPOT taufiq
 *
 *  Yang diubah: nama berkas sesi, kunci di dalam berkas itu, awalan setiap
 *  bidang di dalamnya, dan nama sesi di include/tenantkey.php. Kunci enkripsi
 *  ikut dipindahkan, jadi password router tetap bisa dibuka.
 *
 *  Berkas lama dan berkas kunci dicadangkan lebih dulu.
 */

if (PHP_SAPI !== 'cli') {
  header('HTTP/1.1 403 Forbidden');
  echo "Alat ini hanya bisa dijalankan dari baris perintah.\n";
  exit(1);
}

$root = dirname(dirname(__FILE__));
$opt = array();
$pos = array();
foreach (array_slice($argv, 1) as $arg) {
  if (substr($arg, 0, 2) === '--') {
    $kv = explode('=', substr($arg, 2), 2);
    $opt[$kv[0]] = isset($kv[1]) ? $kv[1] : true;
  } else {
    $pos[] = $arg;
  }
}

if (isset($opt['help']) || isset($opt['h']) || count($pos) < 2) {
  echo "Mikhmon Nocify - ganti nama sesi router\n\n";
  echo "  php tools/mikhmon-rename-session.php LAMA BARU\n\n";
  echo "  LAMA  nama sesi yang sekarang, mis. HOTSPOT\n";
  echo "  BARU  nama yang diinginkan, huruf kecil, mis. taufiq\n";
  echo "        (harus sama dengan subdomain panelnya)\n";
  exit(count($pos) < 2 && !isset($opt['help']) && !isset($opt['h']) ? 1 : 0);
}

$lama = $pos[0];
$baru = strtolower($pos[1]);

$sessDir  = isset($opt['sessions-dir']) ? rtrim($opt['sessions-dir'], '/\\') : $root . '/include/sessions';
$keyFile  = isset($opt['keyfile']) ? $opt['keyfile'] : $root . '/include/tenantkey.php';

$fileLama = $sessDir . '/' . $lama . '.php';
$fileBaru = $sessDir . '/' . $baru . '.php';

if (!preg_match('/^[a-z0-9-]+$/', $baru)) {
  echo "Nama baru harus huruf kecil, angka, dan tanda hubung saja (dipakai sebagai subdomain).\n";
  exit(1);
}
if (!is_file($fileLama)) {
  echo "Tidak ada sesi bernama \"$lama\" di $sessDir\n";
  $ada = array();
  foreach (glob($sessDir . '/*.php') as $f) {
    if (substr($f, -8) !== '.php.bak') {
      $ada[] = basename($f, '.php');
    }
  }
  echo $ada ? "Yang ada: " . implode(', ', $ada) . "\n" : "Belum ada berkas sesi sama sekali.\n";
  exit(1);
}
if (is_file($fileBaru)) {
  echo "Sesi bernama \"$baru\" sudah ada. Pilih nama lain, atau hapus dulu yang itu.\n";
  exit(1);
}

// --- baca sesi lama -------------------------------------------------------
$data = array();
include $fileLama;
if (!isset($data[$lama]) || !is_array($data[$lama])) {
  echo "Berkas $fileLama tidak memuat \$data['$lama'].\n";
  exit(1);
}
$bidang = $data[$lama];

// --- ganti awalan setiap bidang ------------------------------------------
// Setiap nilai berbentuk 'NAMA!isi', 'NAMA@|@isi', dan seterusnya. readcfg
// hanya mengambil bagian setelah pemisahnya, tapi awalannya ikut diganti
// supaya berkasnya tetap konsisten dan bisa dibaca kalau ada yang menyuntingnya.
$bidangBaru = array();
foreach ($bidang as $kunci => $nilai) {
  $bidangBaru[$kunci] = preg_replace('/^' . preg_quote($lama, '/') . '/', $baru, (string) $nilai);
}

// --- tulis berkas baru ----------------------------------------------------
/*
 * Kunci array harus ditulis eksplisit.
 *
 * readcfg.php membaca $data[$sesi][1] sampai [11], sedangkan susunan aslinya
 * hanya menyebut kunci untuk elemen pertama ('1'=>...) dan sisanya mengandalkan
 * penomoran otomatis PHP (jadi 2..11). Kalau ditulis tanpa kunci sama sekali,
 * penomorannya mulai dari 0 dan SELURUH pembacaan bergeser satu bidang -
 * password terbaca sebagai nama hotspot, dan panelnya gagal menyambung ke
 * router. Karena itu kuncinya disalin apa adanya dari berkas asal.
 */
$elemen = array();
foreach ($bidang as $kunci => $nilai) {
  $elemen[] = "'" . $kunci . "'=>'" . $bidangBaru[$kunci] . "'";
}

$isi = "<?php\n"
     . "/* Sesi router satu pelanggan. Dibuat otomatis oleh Mikhmon, jangan di-commit. */\n"
     . "if (isset(\$_SERVER['REQUEST_URI']) && substr(\$_SERVER['REQUEST_URI'], -" . (strlen($baru) + 4) . ") == '" . $baru . ".php') { header('Location:./'); };\n"
     . "\$data['" . $baru . "'] = array (" . implode(',', $elemen) . ");\n";

$cadangan = $fileLama . '.bak-' . date('Ymd-His');
if (!@copy($fileLama, $cadangan)) {
  echo "Gagal membuat cadangan $cadangan\n";
  exit(1);
}

if (!@file_put_contents($fileBaru, $isi)) {
  echo "Gagal menulis $fileBaru\n";
  exit(1);
}
@chmod($fileBaru, 0644);

// --- periksa hasilnya sendiri --------------------------------------------
/*
 * Pembacaan ulang ini bukan hiasan: kalau kunci array bergeser satu saja,
 * seluruh bidang terbaca salah dan panelnya gagal menyambung ke router tanpa
 * pesan yang jelas. Lebih baik gagal di sini, sambil berkas lama masih ada.
 */
$data = array();
include $fileBaru;
$terbaca = isset($data[$baru]) && is_array($data[$baru]) ? $data[$baru] : array();
$salah = array();
foreach ($bidangBaru as $kunci => $harusnya) {
  if (!isset($terbaca[$kunci]) || (string) $terbaca[$kunci] !== (string) $harusnya) {
    $salah[] = $kunci;
  }
}
if (!$salah) {
  // Cek juga dengan cara readcfg membacanya, supaya yakin posisinya benar.
  $pos = array();
  foreach (array('!', '@|@', '#|#', '%', '^', '&', '*', '(', ')', '=', '@!@') as $i => $pemisah) {
    $bagian = explode($pemisah, isset($terbaca[$i + 1]) ? (string) $terbaca[$i + 1] : '');
    $pos[] = isset($bagian[1]) ? $bagian[1] : '';
  }
  if ($pos[0] === '' || $pos[1] === '' || $pos[2] === '') {
    $salah[] = 'posisi bidang';
  }
}
if ($salah) {
  echo "\nGAGAL memeriksa hasil: bidang " . implode(', ', $salah) . " tidak utuh.\n";
  echo "Berkas baru sudah dibuang lagi supaya tidak ada yang rusak.\n";
  echo "Berkas lama Anda masih utuh: $fileLama\n";
  @unlink($fileBaru);
  exit(1);
}
echo "periksa  : 11 bidang utuh dan posisinya benar\n";

echo "sesi     : $lama -> $baru\n";
echo "berkas   : $fileBaru\n";
echo "cadangan : $cadangan\n";
echo "bidang   : " . count($bidangBaru) . " ikut diganti awalannya\n";

// --- pindahkan kunci enkripsi --------------------------------------------
// Kalau langkah ini terlewat, password router tidak akan bisa dibuka lagi.
if (is_file($keyFile)) {
  $mikhmon_session_keys = array();
  include $keyFile;
  if (isset($mikhmon_session_keys[$lama])) {
    if (!@copy($keyFile, $keyFile . '.bak-' . date('Ymd-His'))) {
      echo "Gagal mencadangkan $keyFile - kunci tidak dipindahkan.\n";
      echo "Berkas sesi baru sudah dibuat, jangan dihapus yang lama sebelum ini beres.\n";
      exit(1);
    }
    $mikhmon_session_keys[$baru] = $mikhmon_session_keys[$lama];
    unset($mikhmon_session_keys[$lama]);

    $kunci = "<?php\n"
           . "/*\n"
           . " * Kunci enkripsi per sesi router. Dibuat otomatis oleh\n"
           . " * tools/mikhmon-rekey.php. Jangan di-commit (lihat .gitignore);\n"
           . " * contoh dan penjelasannya ada di include/tenantkey.example.php.\n"
           . " */\n"
           . "if (isset(\$_SERVER['REQUEST_URI']) && substr(\$_SERVER['REQUEST_URI'], -14) == 'tenantkey.php') { header('Location:./'); }\n\n"
           . "\$mikhmon_session_keys = array(\n";
    foreach ($mikhmon_session_keys as $n => $k) {
      $kunci .= "  '" . $n . "' => '" . $k . "',\n";
    }
    $kunci .= ");\n";

    if (!@file_put_contents($keyFile, $kunci)) {
      echo "Gagal menulis $keyFile - kunci tidak dipindahkan.\n";
      exit(1);
    }
    @chmod($keyFile, 0600);
    echo "kunci    : dipindahkan ke nama sesi baru\n";
  } else {
    echo "kunci    : tidak ada kunci untuk sesi \"$lama\" (belum di-rekey)\n";
  }
} else {
  echo "kunci    : $keyFile belum ada (belum di-rekey)\n";
}

// --- hapus berkas lama ----------------------------------------------------
if (@unlink($fileLama)) {
  echo "berkas lama dihapus\n";
} else {
  echo "PERHATIAN: berkas lama gagal dihapus, hapus sendiri: $fileLama\n";
}

echo "\nSelesai. Buka panelnya lewat <" . $baru . ">.nocify.id lalu periksa\n";
echo "Hotspot -> User List masih menampilkan data dari router.\n";
