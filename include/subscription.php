<?php
/*
 *  Mikhmon Nocify - langganan / sewa bulanan.
 *
 *  Copyright (C) 2018 Laksamadi Guko.        (Mikhmon asli)
 *  Copyright (C) 2024 NOCIFY.                (tambahan fork ini)
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation; either version 2 of the License, or
 *  (at your option) any later version.
 *
 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 */

if (defined('MIKHMON_SUBSCRIPTION_LOADED')) {
  return;
}
define('MIKHMON_SUBSCRIPTION_LOADED', 1);

/* =====================================================================
 *  PENGATURAN - silakan ubah bagian ini
 * ===================================================================== */

/* Kunci rahasia untuk menandatangani kode lisensi. Nilai sebenarnya sebaiknya
 * TIDAK ikut ter-commit, karena repo ini publik. Buat file
 * include/license-secret.php berisi satu baris:
 *
 *   <?php define('MIKHMON_LICENSE_SECRET', 'kunci-rahasia-anda');
 *
 * File itu sudah masuk .gitignore. File yang sama harus ada di setiap
 * instalasi pelanggan, karena panel memakai kunci ini untuk MEMERIKSA kode
 * lisensi. Kalau file itu tidak ada, dipakai nilai bawaan di bawah - dan nilai
 * bawaan itu publik, jadi siapa pun bisa membuat kode lisensinya sendiri.
 * Ganti sebelum mulai menjual, dan jangan pernah mengubahnya setelah kode
 * lisensi beredar: semua kode lama langsung tidak berlaku. */
if (!defined('MIKHMON_LICENSE_SECRET')) {
  $mikhmon_secret_file = dirname(__FILE__) . '/license-secret.php';
  if (is_file($mikhmon_secret_file)) {
    include_once $mikhmon_secret_file;
  }
}
if (!defined('MIKHMON_LICENSE_SECRET')) {
  define('MIKHMON_LICENSE_SECRET', 'nocify-mikhmon-ganti-kunci-rahasia-ini');
}

/* QRIS merchant yang ditampilkan di halaman pembayaran. */
if (!defined('MIKHMON_QRIS_MERCHANT')) {
  define('MIKHMON_QRIS_MERCHANT', 'SETIYO ARYO WINATA, DIGITAL & KREATIF');
}
if (!defined('MIKHMON_QRIS_NMID')) {
  define('MIKHMON_QRIS_NMID', 'ID1026599320839');
}

/* Nomor WhatsApp yang dihubungi pelanggan (format internasional, tanpa +). */
if (!defined('MIKHMON_WA_NUMBER')) {
  define('MIKHMON_WA_NUMBER', '6285139495106');
}

/* Mulai memperingatkan berapa hari sebelum kedaluwarsa. */
if (!defined('MIKHMON_LICENSE_WARN_DAYS')) {
  define('MIKHMON_LICENSE_WARN_DAYS', 7);
}

/* Kunci panel kalau lisensi sudah kedaluwarsa.
 * false = hanya tampilkan peringatan, panel tetap bisa dipakai. */
if (!defined('MIKHMON_LICENSE_ENFORCE')) {
  define('MIKHMON_LICENSE_ENFORCE', true);
}

/* Lama tenggang setelah kedaluwarsa sebelum panel benar-benar dikunci. */
if (!defined('MIKHMON_LICENSE_GRACE_DAYS')) {
  define('MIKHMON_LICENSE_GRACE_DAYS', 3);
}

/**
 * Daftar paket sewa.
 *   label  : nama yang tampil
 *   months : lama perpanjangan
 *   price  : harga rupiah. 0 = tampil "Hubungi Admin" (isi harga di sini).
 *   sale   : false = tidak ditawarkan di halaman pembayaran (dipakai kunci
 *            khusus untuk instalasi sendiri).
 */
function mikhmon_license_plans() {
  static $plans = null;
  if ($plans !== null) {
    return $plans;
  }
  $plans = array(
    'P1M'  => array('code' => 'P1M',  'label' => '1 Bulan',   'months' => 1,  'price' => 0,     'sale' => true),
    'P3M'  => array('code' => 'P3M',  'label' => '3 Bulan',   'months' => 3,  'price' => 0,     'sale' => true),
    'P6M'  => array('code' => 'P6M',  'label' => '6 Bulan',   'months' => 6,  'price' => 0,     'sale' => true),
    'P12M' => array('code' => 'P12M', 'label' => '12 Bulan',  'months' => 12, 'price' => 0,     'sale' => true),
    'LIFE' => array('code' => 'LIFE', 'label' => 'Permanen',  'months' => 0,  'price' => 0,     'sale' => false),
  );
  return $plans;
}

/* =====================================================================
 *  Bagian dalam - umumnya tidak perlu diubah
 * ===================================================================== */

/* Cache per request, direset lewat mikhmon_license_reset(). */
$GLOBALS['mikhmon_license_data'] = null;
$GLOBALS['mikhmon_license_status'] = null;
$GLOBALS['mikhmon_license_writable'] = null;

/** Lokasi file lisensi. */
function mikhmon_license_path() {
  return dirname(__FILE__) . '/license.php';
}

function mikhmon_license_reset() {
  $GLOBALS['mikhmon_license_data'] = null;
  $GLOBALS['mikhmon_license_status'] = null;
}

/** true kalau data lisensi benar-benar bisa disimpan. */
function mikhmon_license_writable() {
  mikhmon_license_data();
  return !empty($GLOBALS['mikhmon_license_writable']);
}

function mikhmon_license_new_install_id() {
  if (function_exists('random_bytes')) {
    try {
      return strtoupper(substr(bin2hex(random_bytes(8)), 0, 12));
    } catch (Exception $e) {
      /* fallthrough ke cara lama */
    }
  }
  return strtoupper(substr(md5(uniqid('', true) . mt_rand()), 0, 12));
}

/** Tulis file lisensi dengan pola temp + rename supaya tidak pernah setengah jadi. */
function mikhmon_license_write($data) {
  $path = mikhmon_license_path();
  $body = "<?php\n"
        . "/* Mikhmon Nocify - data langganan. Dikelola otomatis oleh panel. */\n"
        . '$mikhmon_license = ' . var_export($data, true) . ";\n";
  $tmp = $path . '.' . getmypid() . '.' . mt_rand(1000, 9999) . '.tmp';
  if (@file_put_contents($tmp, $body) === strlen($body)) {
    if (@rename($tmp, $path)) {
      return true;
    }
  }
  @unlink($tmp);
  /* Cadangan kalau folder tidak mengizinkan rename. Panjangnya juga diperiksa:
   * tulisan yang terpotong tidak boleh dipakai, karena file lisensi yang
   * setengah jadi akan membuat semua halaman gagal. */
  return @file_put_contents($path, $body) === strlen($body);
}

/** Baca data lisensi. Kalau belum ada, buat ID instalasi baru. */
function mikhmon_license_data() {
  if (is_array($GLOBALS['mikhmon_license_data'])) {
    return $GLOBALS['mikhmon_license_data'];
  }

  $data = array('install_id' => '', 'key' => '', 'plan' => '', 'expires' => '', 'activated' => '');

  $path = mikhmon_license_path();
  if (is_file($path)) {
    $mikhmon_license = null;
    include $path;
    if (isset($mikhmon_license) && is_array($mikhmon_license)) {
      foreach ($data as $k => $v) {
        if (isset($mikhmon_license[$k]) && is_scalar($mikhmon_license[$k])) {
          $data[$k] = (string) $mikhmon_license[$k];
        }
      }
    }
  }

  if (!preg_match('/^[A-Z0-9]{12}$/', $data['install_id'])) {
    $data['install_id'] = mikhmon_license_new_install_id();
    $GLOBALS['mikhmon_license_writable'] = mikhmon_license_write($data);
  } else {
    /* temp+rename butuh folder yang bisa ditulis; fallback in-place butuh filenya. */
    $GLOBALS['mikhmon_license_writable'] = is_writable(dirname($path)) || is_writable($path);
  }

  $GLOBALS['mikhmon_license_data'] = $data;
  return $data;
}

function mikhmon_license_sign($expiry, $plan, $bind) {
  return strtoupper(substr(hash_hmac('sha256', $expiry . '|' . $plan . '|' . $bind, MIKHMON_LICENSE_SECRET), 0, 8));
}

function mikhmon_license_sig_match($a, $b) {
  if (function_exists('hash_equals')) {
    return hash_equals($a, $b);
  }
  return $a === $b;
}

/**
 * Periksa kode lisensi. Format: MKN-YYYYMMDD-PAKET-SIGNATURE
 * Balikannya array data lisensi, atau string alasan kalau ditolak.
 */
function mikhmon_license_parse($key, $install_id) {
  $key = strtoupper(trim((string) $key));
  $key = preg_replace('/[^A-Z0-9\-]/', '', $key);
  $parts = explode('-', $key, 4);
  if (count($parts) != 4) {
    return 'format';
  }
  list($tag, $expiry, $plan, $sig) = $parts;
  if ($tag != 'MKN') {
    return 'format';
  }
  if (!preg_match('/^\d{8}$/', $expiry) || !preg_match('/^[A-Z0-9]{8}$/', $sig)) {
    return 'format';
  }
  $plans = mikhmon_license_plans();
  if (!isset($plans[$plan])) {
    return 'paket';
  }
  if (!mikhmon_license_sig_match(mikhmon_license_sign($expiry, $plan, $install_id), $sig)
      && !mikhmon_license_sig_match(mikhmon_license_sign($expiry, $plan, '*'), $sig)) {
    return 'tanda-tangan';
  }
  $y = (int) substr($expiry, 0, 4);
  $m = (int) substr($expiry, 4, 2);
  $d = (int) substr($expiry, 6, 2);
  if (!checkdate($m, $d, $y)) {
    return 'format';
  }
  return array(
    'key'     => 'MKN-' . $expiry . '-' . $plan . '-' . $sig,
    'plan'    => $plan,
    'expires' => sprintf('%04d-%02d-%02d', $y, $m, $d),
  );
}

/** Simpan kode lisensi yang baru. Balikannya array(status, hasil). */
function mikhmon_license_apply($key) {
  $data = mikhmon_license_data();
  $parsed = mikhmon_license_parse($key, $data['install_id']);
  if (!is_array($parsed)) {
    return array(false, $parsed);
  }
  $data['key'] = $parsed['key'];
  $data['plan'] = $parsed['plan'];
  $data['expires'] = $parsed['expires'];
  $data['activated'] = date('Y-m-d');
  if (!mikhmon_license_write($data)) {
    return array(false, 'gagal-tulis');
  }
  mikhmon_license_reset();
  return array(true, $parsed);
}

function mikhmon_license_clear() {
  $data = mikhmon_license_data();
  $data['key'] = '';
  $data['plan'] = '';
  $data['expires'] = '';
  $data['activated'] = '';
  $ok = mikhmon_license_write($data);
  mikhmon_license_reset();
  return $ok;
}

/**
 * Status langganan.
 *   state : trial | active | warning | grace | expired
 *   days  : sisa hari (negatif kalau sudah lewat)
 */
function mikhmon_license_status() {
  if (is_array($GLOBALS['mikhmon_license_status'])) {
    return $GLOBALS['mikhmon_license_status'];
  }

  $data = mikhmon_license_data();
  $plans = mikhmon_license_plans();
  $today = strtotime(date('Y-m-d'));

  $out = array(
    'state'        => 'trial',
    'plan'         => '',
    'plan_label'   => '',
    'expires'      => '',
    'days'         => null,
    'install_id'   => $data['install_id'],
    'key'          => '',
    'key_short'    => '',
    'activated'    => $data['activated'],
    'licensed'     => false,
    'lifetime'     => false,
    'active'       => true,
    'grace_days'   => MIKHMON_LICENSE_GRACE_DAYS,
  );

  if ($data['key'] == '' || $data['expires'] == '') {
    $GLOBALS['mikhmon_license_status'] = $out;
    return $out;
  }

  /* Tanggal harus berbentuk YYYY-MM-DD dan tanggal yang sah. Nilai rusak
   * dianggap "tanpa lisensi", bukan "kedaluwarsa", supaya file yang salah
   * diedit tidak mengunci panel. */
  $m = array();
  if (!preg_match('/^(\d{4})-(\d{2})-(\d{2})$/', $data['expires'], $m)
      || !checkdate((int) $m[2], (int) $m[3], (int) $m[1])) {
    $GLOBALS['mikhmon_license_status'] = $out;
    return $out;
  }

  /* Lisensi permanen tidak pernah dikonversi ke timestamp: strtotime() tahun
   * 9999 gagal di PHP 32-bit dan akan berbalik jadi "kedaluwarsa". */
  if ($data['plan'] == 'LIFE') {
    $days = 36500;
    $out['lifetime'] = true;
  } else {
    $days = (int) round((strtotime($data['expires']) - $today) / 86400);
  }

  $out['licensed'] = true;
  $out['plan'] = $data['plan'];
  $out['plan_label'] = isset($plans[$data['plan']]) ? $plans[$data['plan']]['label'] : $data['plan'];
  $out['expires'] = $data['expires'];
  $out['key'] = $data['key'];
  $out['key_short'] = 'MKN-•••-' . $data['plan'] . '-' . substr($data['key'], -4);
  $out['days'] = $days;

  if ($days < 0) {
    $out['state'] = (-$days <= MIKHMON_LICENSE_GRACE_DAYS) ? 'grace' : 'expired';
    $out['active'] = false;
  } elseif ($days <= MIKHMON_LICENSE_WARN_DAYS) {
    $out['state'] = 'warning';
  } else {
    $out['state'] = 'active';
  }

  $GLOBALS['mikhmon_license_status'] = $out;
  return $out;
}

/** true kalau panel harus dikunci. */
function mikhmon_license_locked() {
  if (!MIKHMON_LICENSE_ENFORCE) {
    return false;
  }
  $st = mikhmon_license_status();
  return ($st['state'] == 'expired');
}

/** true kalau perlu ditampilkan peringatan di atas halaman. */
function mikhmon_license_warn() {
  $st = mikhmon_license_status();
  return in_array($st['state'], array('warning', 'grace', 'expired'));
}

/** 1234ABCD5678 -> 1234-ABCD-5678 */
function mikhmon_license_pretty_id($id) {
  $id = (string) $id;
  if (strlen($id) != 12) {
    return $id;
  }
  return substr($id, 0, 4) . '-' . substr($id, 4, 4) . '-' . substr($id, 8, 4);
}

function mikhmon_license_price($price) {
  if (!$price) {
    return '';
  }
  return 'Rp ' . number_format((float) $price, 0, ',', '.');
}

function mikhmon_wa_link($text = '') {
  $url = 'https://wa.me/' . MIKHMON_WA_NUMBER;
  if ($text !== '') {
    $url .= '?text=' . rawurlencode($text);
  }
  return $url;
}

/** Nama file gambar QRIS kalau ada, dalam bentuk URL yang bisa dipakai <img>. */
function mikhmon_qris_image_url() {
  $dir = dirname(__FILE__) . '/../img/';
  foreach (array('png', 'jpg', 'jpeg', 'webp') as $ext) {
    if (is_file($dir . 'qris-merchant.' . $ext)) {
      return './img/qris-merchant.' . $ext . '?t=' . filemtime($dir . 'qris-merchant.' . $ext);
    }
  }
  return '';
}

/** Pita peringatan yang tampil di atas semua halaman. */
function mikhmon_license_banner() {
  if (!mikhmon_license_warn()) {
    return '';
  }
  $st = mikhmon_license_status();

  if ($st['state'] == 'expired') {
    return '<div class="box bg-danger" style="margin:0;border-radius:0;text-align:center">'
         . '<i class="fa fa-lock"></i> <b>Langganan Mikhmon sudah berakhir.</b> '
         . 'Panel dikunci sampai lisensi diperpanjang. '
         . '<a href="./admin.php?id=subscription" style="color:#fff;text-decoration:underline">Perpanjang sekarang</a>'
         . '</div>';
  }
  if ($st['state'] == 'grace') {
    return '<div class="box bg-warning" style="margin:0;border-radius:0;text-align:center">'
         . '<i class="fa fa-exclamation-triangle"></i> <b>Langganan sudah lewat '
         . htmlspecialchars(abs((int) $st['days']), ENT_QUOTES) . ' hari.</b> '
         . 'Masa tenggang ' . (int) MIKHMON_LICENSE_GRACE_DAYS . ' hari. '
         . '<a href="./admin.php?id=subscription">Perpanjang sekarang</a>'
         . '</div>';
  }
  return '<div class="box bg-warning" style="margin:0;border-radius:0;text-align:center">'
       . '<i class="fa fa-clock-o"></i> Langganan Mikhmon berakhir dalam <b>'
       . (int) $st['days'] . ' hari</b> (' . htmlspecialchars($st['expires'], ENT_QUOTES) . '). '
       . '<a href="./admin.php?id=subscription">Lihat langganan</a>'
       . '</div>';
}

/**
 * Simpan gambar QRIS dari form unggah.
 * Balikannya pesan galat, atau string kosong kalau berhasil.
 */
function mikhmon_qris_upload() {
  if (!isset($_FILES['QRIS']) || !is_uploaded_file($_FILES['QRIS']['tmp_name'])) {
    return 'Tidak ada file yang diunggah, atau file terlalu besar untuk server.';
  }
  $file = $_FILES['QRIS'];
  if ($file['error'] != UPLOAD_ERR_OK) {
    return 'Unggahan gagal (kode ' . (int) $file['error'] . '). Coba file yang lebih kecil.';
  }
  if ($file['size'] > 2 * 1024 * 1024) {
    return 'Ukuran file lebih dari 2 MB.';
  }
  $ext = strtolower(pathinfo($file['name'], PATHINFO_EXTENSION));
  if (!in_array($ext, array('png', 'jpg', 'jpeg', 'webp'))) {
    return 'Hanya file PNG, JPG, atau WEBP yang diizinkan.';
  }
  /* Cek isi filenya benar-benar gambar, jangan cuma percaya ekstensi. */
  if (@getimagesize($file['tmp_name']) === false) {
    return 'File itu bukan gambar yang sah.';
  }
  $dir = dirname(__FILE__) . '/../img/';
  if (!is_dir($dir) || !is_writable($dir)) {
    return 'Folder img/ tidak bisa ditulis. Periksa izin tulis foldernya.';
  }
  $target = $dir . 'qris-merchant.' . $ext;
  if (!@move_uploaded_file($file['tmp_name'], $target)) {
    return 'Gambar gagal dipindahkan ke folder img/.';
  }
  @chmod($target, 0644);
  /* Gambar lama baru dihapus setelah yang baru benar-benar tersimpan. */
  foreach (array('png', 'jpg', 'jpeg', 'webp') as $other) {
    if ($other != $ext) {
      @unlink($dir . 'qris-merchant.' . $other);
    }
  }
  return '';
}

function mikhmon_qris_remove() {
  $dir = dirname(__FILE__) . '/../img/';
  $removed = false;
  foreach (array('png', 'jpg', 'jpeg', 'webp') as $ext) {
    if (is_file($dir . 'qris-merchant.' . $ext)) {
      if (@unlink($dir . 'qris-merchant.' . $ext)) {
        $removed = true;
      }
    }
  }
  return $removed;
}

/* --------------------------------------------------------- token CSRF ---
 * Panel ini tidak punya kerangka token sendiri, sedangkan tiga aksi di
 * halaman langganan bisa dipicu halaman lain lewat POST biasa (termasuk
 * menghapus lisensi). Jadi ketiganya memakai token per sesi. */

function mikhmon_sub_token() {
  if (empty($_SESSION['mikhmon_sub_token'])) {
    $_SESSION['mikhmon_sub_token'] = function_exists('random_bytes')
      ? bin2hex(random_bytes(16))
      : md5(uniqid('', true) . mt_rand());
  }
  return $_SESSION['mikhmon_sub_token'];
}

function mikhmon_sub_token_ok() {
  $sent = isset($_POST['sub_token']) ? (string) $_POST['sub_token'] : '';
  $mine = isset($_SESSION['mikhmon_sub_token']) ? (string) $_SESSION['mikhmon_sub_token'] : '';
  if ($mine === '' || $sent === '') {
    return false;
  }
  if (function_exists('hash_equals')) {
    return hash_equals($mine, $sent);
  }
  return $mine === $sent;
}
