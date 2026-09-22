#!/usr/bin/env php
<?php
/*
 *  Mikhmon Nocify - pembuat kode lisensi langganan.
 *
 *  Copyright (C) 2024 NOCIFY.
 *  GPLv2.
 *
 *  ALAT INI UNTUK PENJUAL - JANGAN DIUNGGAH KE WEB ROOT YANG BISA DIAKSES PUBLIK.
 *  Alat ini memegang kunci rahasia, jadi siapa pun yang bisa menjalankannya
 *  bisa membuat lisensi sendiri. Letakkan hanya di komputer/VPS Anda, dan
 *  sebaiknya jangan ikut ter-deploy ke server pelanggan.
 *
 *  Cara pakai (jalankan di folder utama Mikhmon):
 *
 *    php tools/mikhmon-keygen.php --list
 *    php tools/mikhmon-keygen.php --id=A1B2-C3D4-E5F6 --plan=P1M
 *    php tools/mikhmon-keygen.php --id=A1B2C3D4E5F6 --plan=P3M --from=2025-01-01
 *    php tools/mikhmon-keygen.php --id=* --plan=LIFE
 *    php tools/mikhmon-keygen.php --check=MKN-20251231-P1M-XXXXXXXX --id=A1B2C3D4E5F6
 *
 *  --id=*  membuat kode yang berlaku di instalasi mana pun (untuk instalasi
 *  Anda sendiri atau saat pelanggan tidak bisa menyebutkan ID instalasinya).
 */

if (PHP_SAPI !== 'cli') {
  header('HTTP/1.1 403 Forbidden');
  echo "This tool can only be run from the command line.\n";
  exit(1);
}

require_once dirname(__FILE__) . '/../include/subscription.php';

$opt = array();
foreach (array_slice($argv, 1) as $arg) {
  if (substr($arg, 0, 2) == '--') {
    $kv = explode('=', substr($arg, 2), 2);
    $opt[$kv[0]] = isset($kv[1]) ? $kv[1] : true;
  } else {
    $opt['_'][] = $arg;
  }
}

function keygen_usage() {
  echo "Mikhmon Nocify - pembuat kode lisensi\n\n";
  echo "  --list                     lihat daftar paket\n";
  echo "  --id=XXXXXXXXXXXX          ID instalasi pelanggan (12 karakter, boleh pakai tanda -)\n";
  echo "                             pakai --id=* untuk kode yang berlaku di mana saja\n";
  echo "  --plan=P1M                 kode paket, lihat --list\n";
  echo "  --from=YYYY-MM-DD          tanggal mulai (bawaan: hari ini)\n";
  echo "  --extend=YYYY-MM-DD        perpanjang dari tanggal ini, bukan dari hari ini\n";
  echo "  --check=KODE --id=...      periksa apakah sebuah kode sah untuk instalasi itu\n\n";
  echo "Contoh:\n";
  echo "  php tools/mikhmon-keygen.php --id=A1B2C3D4E5F6 --plan=P1M\n";
  echo "  php tools/mikhmon-keygen.php --check=MKN-20251231-P1M-A1B2C3D4 --id=A1B2C3D4E5F6\n";
}

$plans = mikhmon_license_plans();

/* ------------------------------------------------------------ --list */
if (isset($opt['list'])) {
  echo "Daftar paket:\n";
  foreach ($plans as $p) {
    printf("  %-5s  %-10s  %s%s\n",
      $p['code'],
      $p['label'],
      $p['price'] > 0 ? mikhmon_license_price($p['price']) : 'harga belum diisi',
      $p['sale'] ? '' : '   (tidak dijual, khusus)');
  }
  echo "\nHarga diatur di include/subscription.php, fungsi mikhmon_license_plans().\n";
  exit(0);
}

/* ----------------------------------------------------------- --check */
if (isset($opt['check'])) {
  $id = isset($opt['id']) ? $opt['id'] : '';
  $id = preg_replace('/[^A-Za-z0-9*]/', '', $id);
  $id = strtoupper($id);
  $res = mikhmon_license_parse($opt['check'], $id);
  if (is_array($res)) {
    echo "SAH\n";
    echo "  paket   : " . $res['plan'] . "\n";
    echo "  berlaku : " . $res['expires'] . "\n";
    echo "  kode    : " . $res['key'] . "\n";
    exit(0);
  }
  echo "TIDAK SAH (" . $res . ")\n";
  exit(1);
}

/* ---------------------------------------------------------- generate */
if (!isset($opt['id']) || !isset($opt['plan'])) {
  keygen_usage();
  exit(1);
}

$id = preg_replace('/[^A-Za-z0-9*]/', '', $opt['id']);
$id = strtoupper($id);
if ($id !== '*' && !preg_match('/^[A-Z0-9]{12}$/', $id)) {
  echo "ID instalasi harus 12 karakter huruf/angka (contoh A1B2-C3D4-E5F6), atau * .\n";
  exit(1);
}

$plan = strtoupper($opt['plan']);
if (!isset($plans[$plan])) {
  echo "Paket '$plan' tidak dikenal. Jalankan --list untuk melihat daftar paket.\n";
  exit(1);
}

/**
 * Tambah bulan dengan menjepit ke hari terakhir bulan tujuan.
 * Penjumlahan bulan bawaan PHP melewati akhir bulan (31 Jan + 1 bulan jatuh
 * di 3 Maret), jadi pelanggan bisa dapat beberapa hari gratis.
 */
function keygen_add_months($ymd, $months) {
  list($y, $m, $d) = array_map('intval', explode('-', $ymd));
  $m += (int) $months;
  $y += (int) floor(($m - 1) / 12);
  $m = (($m - 1) % 12) + 1;
  $last = (int) date('t', mktime(12, 0, 0, $m, 1, $y));
  if ($d > $last) {
    $d = $last;
  }
  return sprintf('%04d-%02d-%02d', $y, $m, $d);
}

$base = date('Y-m-d');
foreach (array('extend', 'from') as $src) {
  if (isset($opt[$src]) && $opt[$src] !== true) {
    $ts = strtotime($opt[$src]);
    if ($ts === false) {
      echo "Tanggal --" . $src . " tidak bisa dibaca: '" . $opt[$src] . "'. Pakai format YYYY-MM-DD.\n";
      exit(1);
    }
    $base = date('Y-m-d', $ts);
    break;
  }
}

if ($plans[$plan]['months'] <= 0) {
  $expiry = '9999-12-31';
} else {
  $expiry = keygen_add_months($base, $plans[$plan]['months']);
}

$expiry_compact = str_replace('-', '', $expiry);
$sig = mikhmon_license_sign($expiry_compact, $plan, $id);
$key = 'MKN-' . $expiry_compact . '-' . $plan . '-' . $sig;

$days = ($plans[$plan]['months'] <= 0) ? 0 : (int) round((strtotime($expiry) - strtotime(date('Y-m-d'))) / 86400);

echo "Kode lisensi Mikhmon Nocify\n";
echo "-----------------------------------------------\n";
echo "  Untuk instalasi : " . ($id === '*' ? 'SEMUA instalasi (*)' : mikhmon_license_pretty_id($id)) . "\n";
echo "  Paket           : " . $plans[$plan]['label'] . "\n";
echo "  Mulai           : " . $base . "\n";
echo "  Berlaku sampai  : " . $expiry . ($plans[$plan]['months'] <= 0 ? "  (permanen)" : "  (" . $days . " hari dari hari ini)") . "\n";
echo "-----------------------------------------------\n";
echo "  " . $key . "\n";
echo "-----------------------------------------------\n";
echo "Kirim kode di atas ke pelanggan. Pelanggan menempelkannya di\n";
echo "menu Langganan -> Aktivasi Lisensi.\n";
