<?php
/*
 *  Mikhmon Nocify - langganan / sewa bulanan lewat portal pusat.
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
 *
 *  Panel ini TIDAK lagi menyimpan kunci lisensi. Setiap beberapa jam panel
 *  bertanya ke portal pusat "apakah instalasi ini masih aktif?", lalu
 *  jawabannya disimpan sebagai cache supaya panel tetap jalan saat internet
 *  mati. Harga, paket, dan QRIS semuanya diatur di portal, jadi tidak ada
 *  rahasia apa pun yang perlu disimpan di server pelanggan.
 */

if (defined('MIKHMON_SUBSCRIPTION_LOADED')) {
  return;
}
define('MIKHMON_SUBSCRIPTION_LOADED', 1);

/* =====================================================================
 *  PENGATURAN - silakan ubah bagian ini
 * ===================================================================== */

/* Jarak antar pemeriksaan ke portal, dalam detik.
 * 21600 = 6 jam. Selama cache masih lebih baru dari ini, panel tidak
 * menghubungi portal sama sekali. */
if (!defined('MIKHMON_HEARTBEAT_INTERVAL')) {
  define('MIKHMON_HEARTBEAT_INTERVAL', 21600);
}

/* Umur maksimal cache, dalam detik. 604800 = 7 hari.
 * Kalau cache sudah lebih tua dari ini (portal lama tidak bisa dihubungi),
 * panel menganggap langganan tidak dapat diverifikasi dan mengunci diri.
 * Instalasi yang belum pernah berhasil menghubungi portal TIDAK pernah
 * dikunci lewat aturan ini. */
if (!defined('MIKHMON_HEARTBEAT_MAX_AGE')) {
  define('MIKHMON_HEARTBEAT_MAX_AGE', 604800);
}

/* Nomor WhatsApp yang dihubungi pelanggan (format internasional, tanpa +). */
if (!defined('MIKHMON_WA_NUMBER')) {
  define('MIKHMON_WA_NUMBER', '6285139495106');
}

/* Mulai memperingatkan berapa hari sebelum kedaluwarsa. Portal yang menentukan
 * status sebenarnya; angka ini hanya dipakai kalau portal bilang "active" tapi
 * sisa harinya sudah mepet. */
if (!defined('MIKHMON_LICENSE_WARN_DAYS')) {
  define('MIKHMON_LICENSE_WARN_DAYS', 7);
}

/* =====================================================================
 *  Bagian dalam - umumnya tidak perlu diubah
 * ===================================================================== */

$GLOBALS['mikhmon_instance_data'] = null;
$GLOBALS['mikhmon_heartbeat_data'] = null;
$GLOBALS['mikhmon_heartbeat_failed'] = false;
$GLOBALS['mikhmon_license_status'] = null;

/** Lokasi file kredensial instalasi. */
function mikhmon_instance_path() {
  return dirname(__FILE__) . '/instance.php';
}

/** Lokasi file cache jawaban portal. */
/*
 * Tenant yang sedang dilayani permintaan ini.
 *
 * Di model hosted bersama, satu panel melayani banyak pelanggan dan portal
 * membedakannya lewat nama tenant. Yang dipakai adalah subdomain
 * (<nama>.nocify.id). Instalasi dedicated tidak punya subdomain, jadi
 * tenant-nya kosong dan portal mencarikan sendiri pelanggannya.
 */
function mikhmon_heartbeat_tenant() {
  if (!function_exists('mikhmon_tenant_session')) {
    return '';
  }
  $tenant = mikhmon_tenant_session();
  if ($tenant === '') {
    return '';
  }
  return preg_replace('/[^a-z0-9-]/', '', strtolower($tenant));
}

/*
 * Cache jawaban portal dipisah per tenant.
 *
 * Kalau semua pelanggan berbagi satu berkas cache, permintaan pelanggan A
 * akan menimpa jawaban untuk pelanggan B - halaman B bisa menampilkan status
 * langganan A. Karena itu berkasnya diberi nama tenant.
 */
function mikhmon_heartbeat_cache_path() {
  $tenant = mikhmon_heartbeat_tenant();
  if ($tenant === '') {
    return dirname(__FILE__) . '/heartbeat-cache.php';
  }
  return dirname(__FILE__) . '/heartbeat-cache-' . $tenant . '.php';
}

/**
 * Baca include/instance.php. Balikannya array('id', 'token', 'portal') yang
 * sudah dibersihkan; nilai kosong kalau file itu tidak ada atau tidak lengkap.
 */
function mikhmon_instance_data() {
  if (is_array($GLOBALS['mikhmon_instance_data'])) {
    return $GLOBALS['mikhmon_instance_data'];
  }

  $data = array('id' => '', 'token' => '', 'portal' => '');
  $path = mikhmon_instance_path();

  if (is_file($path)) {
    $mikhmon_instance = null;
    include $path;
    if (isset($mikhmon_instance) && is_array($mikhmon_instance)) {
      foreach (array_keys($data) as $k) {
        if (isset($mikhmon_instance[$k]) && is_scalar($mikhmon_instance[$k])) {
          $data[$k] = trim((string) $mikhmon_instance[$k]);
        }
      }
    }
  }

  /* Portal harus alamat http/https yang masuk akal; garis miring di akhir
   * dibuang supaya penggabungan URL di bawah tidak menghasilkan "//". */
  if ($data['portal'] != '' && !preg_match('#^https?://[^\s]+$#i', $data['portal'])) {
    $data['portal'] = '';
  }
  $data['portal'] = rtrim($data['portal'], '/');

  if (!preg_match('/^[A-Za-z0-9._-]{4,64}$/', $data['id'])) {
    $data['id'] = '';
  }

  $GLOBALS['mikhmon_instance_data'] = $data;
  return $data;
}

/** true kalau id, token, dan portal ketiganya terisi. */
function mikhmon_instance_configured() {
  $data = mikhmon_instance_data();
  return ($data['id'] != '' && $data['token'] != '' && $data['portal'] != '');
}

/** Versi panel yang dikirim ke portal, misalnya "3.20". */
function mikhmon_heartbeat_version() {
  if (isset($_SESSION['v']) && is_scalar($_SESSION['v'])) {
    if (preg_match('/^v?([0-9]+\.[0-9]+)/', trim((string) $_SESSION['v']), $m)) {
      return $m[1];
    }
  }

  $file = dirname(__FILE__) . '/../verson.txt';
  if (is_file($file)) {
    $raw = @file_get_contents($file);
    if ($raw !== false && preg_match('/"version"\s*:\s*"?v?([0-9]+\.[0-9]+)/', $raw, $m)) {
      return $m[1];
    }
  }

  return '3.20';
}

/** Baca cache heartbeat. Balikannya array('time' => int, 'response' => array) atau null. */
function mikhmon_heartbeat_read_cache() {
  if (is_array($GLOBALS['mikhmon_heartbeat_data'])) {
    return $GLOBALS['mikhmon_heartbeat_data'];
  }

  $path = mikhmon_heartbeat_cache_path();
  if (!is_file($path)) {
    return null;
  }

  $mikhmon_heartbeat = null;
  include $path;

  if (!isset($mikhmon_heartbeat) || !is_array($mikhmon_heartbeat)
      || !isset($mikhmon_heartbeat['time']) || !is_numeric($mikhmon_heartbeat['time'])
      || !isset($mikhmon_heartbeat['response']) || !is_array($mikhmon_heartbeat['response'])) {
    return null;
  }

  $cache = array(
    'time'     => (int) $mikhmon_heartbeat['time'],
    'response' => $mikhmon_heartbeat['response'],
  );
  $GLOBALS['mikhmon_heartbeat_data'] = $cache;
  return $cache;
}

/** Tulis cache lewat file sementara + rename, sama seperti include/readcfg.php. */
function mikhmon_heartbeat_write_cache($cache) {
  $path = mikhmon_heartbeat_cache_path();
  $body = "<?php\n"
        . "/* Mikhmon Nocify - cache heartbeat portal. Dikelola otomatis oleh panel. */\n"
        . '$mikhmon_heartbeat = ' . var_export($cache, true) . ";\n";

  $tmp = $path . '.' . getmypid() . '.' . mt_rand(1000, 9999) . '.tmp';
  if (@file_put_contents($tmp, $body) === strlen($body)) {
    if (@rename($tmp, $path)) {
      return true;
    }
  }
  @unlink($tmp);

  /* Cadangan kalau folder tidak mengizinkan rename; panjangnya diperiksa juga
   * supaya file yang setengah jadi tidak pernah dipakai. */
  return @file_put_contents($path, $body) === strlen($body);
}

/**
 * POST JSON ke portal. Balikannya array hasil decode kalau HTTP 200, atau null
 * kalau koneksi gagal, timeout, atau status bukan 200.
 *
 * Polanya sama dengan mikhmon_api_post() di lib/routeros_api.class.php: cURL
 * kalau ada, kalau tidak file_get_contents + stream context. Bedanya di sini
 * timeout-nya pendek dan status HTTP diperiksa.
 */
function mikhmon_heartbeat_http_post($url, $payload, $timeout_sec) {
  $body = json_encode($payload);
  if ($body === false) {
    return null;
  }

  $headers = array(
    'Content-Type: application/json',
    'Accept: application/json',
  );

  if (function_exists('curl_init')) {
    $ch = curl_init();
    if ($ch === false) {
      return null;
    }
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_POST, true);
    curl_setopt($ch, CURLOPT_URL, $url);
    curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
    curl_setopt($ch, CURLOPT_POSTFIELDS, $body);
    curl_setopt($ch, CURLOPT_CONNECTTIMEOUT, 5);
    curl_setopt($ch, CURLOPT_TIMEOUT, $timeout_sec);
    curl_setopt($ch, CURLOPT_FOLLOWLOCATION, false);

    $out = @curl_exec($ch);
    if ($out === false) {
      @curl_close($ch);
      return null;
    }
    $code = (int) @curl_getinfo($ch, CURLINFO_HTTP_CODE);
    @curl_close($ch);
    if ($code != 200) {
      return null;
    }

    $decoded = json_decode($out, true);
    return is_array($decoded) ? $decoded : null;
  }

  $context = stream_context_create(array('http' => array(
    'method'        => 'POST',
    'header'        => implode("\r\n", $headers) . "\r\n",
    'content'       => $body,
    'timeout'       => $timeout_sec,
    'ignore_errors' => true,
  )));

  $out = @file_get_contents($url, false, $context);
  if ($out === false) {
    return null;
  }

  /* Dengan ignore_errors, jawaban 4xx/5xx tetap terbaca - jadi statusnya harus
   * diperiksa dari baris pertama header. */
  $code = 0;
  if (isset($http_response_header[0]) && preg_match('/\s(\d{3})\s/', $http_response_header[0], $m)) {
    $code = (int) $m[1];
  }
  if ($code != 200) {
    return null;
  }

  $decoded = json_decode($out, true);
  return is_array($decoded) ? $decoded : null;
}

/**
 * Minta status terbaru ke portal, dengan cache.
 *
 * - Tidak dipaksa dan cache masih lebih baru dari MIKHMON_HEARTBEAT_INTERVAL:
 *   langsung balik, tanpa panggilan jaringan.
 * - Berhasil: jawaban disimpan ke include/heartbeat-cache.php.
 * - Gagal (koneksi, timeout, bukan 200): cache lama dipertahankan dan
 *   $GLOBALS['mikhmon_heartbeat_failed'] di-set, sehingga status jadi "stale".
 *
 * Balikannya array('time' => int, 'response' => array) atau null kalau belum
 * ada data sama sekali.
 */
function mikhmon_heartbeat_refresh($force = false) {
  $cache = mikhmon_heartbeat_read_cache();

  if (!$force && is_array($cache)
      && (time() - (int) $cache['time']) < MIKHMON_HEARTBEAT_INTERVAL) {
    return $cache;
  }

  if (!mikhmon_instance_configured()) {
    return $cache;
  }

  $inst = mikhmon_instance_data();
  $payload = array(
    'instance_id' => $inst['id'],
    'token'       => $inst['token'],
    'version'     => mikhmon_heartbeat_version(),
  );
  $tenant = mikhmon_heartbeat_tenant();
  if ($tenant !== '') {
    $payload['tenant'] = $tenant;
  }
  $url = $inst['portal'] . '/api/v1/heartbeat';

  $response = mikhmon_heartbeat_http_post($url, $payload, 10);
  if (!is_array($response)) {
    $GLOBALS['mikhmon_heartbeat_failed'] = true;
    return $cache;
  }

  $new = array('time' => time(), 'response' => $response);
  mikhmon_heartbeat_write_cache($new);

  /* Jawaban baru tetap dipakai walau file cache gagal ditulis, supaya
   * permintaan ini tidak menampilkan data lama. */
  $GLOBALS['mikhmon_heartbeat_data'] = $new;
  $GLOBALS['mikhmon_heartbeat_failed'] = false;
  $GLOBALS['mikhmon_license_status'] = null;

  return $new;
}

/**
 * Status langganan.
 *   state : unconfigured | active | warning | grace | expired
 *   days  : sisa hari (negatif kalau sudah lewat), null kalau tidak diketahui
 *
 * Aturan penting: instalasi yang belum pernah berhasil menghubungi portal
 * (belum ada include/instance.php, atau belum ada cache sama sekali) TIDAK
 * PERNAH dianggap kedaluwarsa. Jadi instalasi baru / portal yang sedang mati
 * tidak bisa mengunci panel sendiri.
 */
function mikhmon_license_status() {
  if (is_array($GLOBALS['mikhmon_license_status'])) {
    return $GLOBALS['mikhmon_license_status'];
  }

  $inst = mikhmon_instance_data();
  $configured = mikhmon_instance_configured();

  $out = array(
    'state'      => 'unconfigured',
    'plan'       => '',
    'plan_label' => '',
    'expires'    => '',
    'days'       => null,
    'install_id' => $inst['id'],
    'licensed'   => false,
    'active'     => true,
    'message'    => $configured
                  ? 'Belum ada data dari portal. Panel tetap bisa dipakai.'
                  : 'Panel ini belum terhubung ke portal langganan.',
    'pay_url'    => '',
    'checked_at' => null,
    'stale'      => true,
    'configured' => $configured,
    'portal'     => $inst['portal'],
  );

  if (!$configured) {
    $GLOBALS['mikhmon_license_status'] = $out;
    return $out;
  }

  /* Sekaligus memicu heartbeat kalau cache sudah waktunya diperbarui. */
  $cache = mikhmon_heartbeat_refresh(false);

  /* Portal belum pernah berhasil dihubungi dan belum ada cache: jangan kunci
   * panel. Sama seperti instalasi yang belum terdaftar. */
  if (!is_array($cache)) {
    $out['message'] = 'Portal langganan belum bisa dihubungi. Panel tetap bisa dipakai.';
    $GLOBALS['mikhmon_license_status'] = $out;
    return $out;
  }

  $resp = $cache['response'];
  $age = time() - (int) $cache['time'];

  $state = isset($resp['state']) ? (string) $resp['state'] : '';
  if (!in_array($state, array('active', 'warning', 'grace', 'expired'))) {
    /* Jawaban yang tidak dikenal jangan sampai mengunci panel. */
    $state = 'active';
  }

  $days = null;
  if (isset($resp['days_left']) && is_numeric($resp['days_left'])) {
    $days = (int) $resp['days_left'];
  } elseif (isset($resp['expires_at']) && is_string($resp['expires_at'])) {
    $ts = strtotime($resp['expires_at']);
    if ($ts !== false) {
      $days = (int) round(($ts - strtotime(date('Y-m-d'))) / 86400);
    }
  }

  if ($state == 'active' && $days !== null && $days <= MIKHMON_LICENSE_WARN_DAYS) {
    $state = 'warning';
  }

  $out['state'] = $state;
  $out['plan'] = isset($resp['plan']) ? (string) $resp['plan'] : '';
  $out['plan_label'] = $out['plan'];
  $out['expires'] = isset($resp['expires_at']) ? (string) $resp['expires_at'] : '';
  $out['days'] = $days;
  $out['licensed'] = true;
  $out['active'] = ($state != 'expired');
  $out['message'] = isset($resp['message']) ? (string) $resp['message'] : '';
  $out['pay_url'] = isset($resp['pay_url']) ? (string) $resp['pay_url'] : '';
  $out['checked_at'] = (int) $cache['time'];
  $out['stale'] = ($age > MIKHMON_HEARTBEAT_INTERVAL * 2)
                || !empty($GLOBALS['mikhmon_heartbeat_failed']);

  /* Terlalu lama tidak bisa diperbarui = langganan tidak dapat diverifikasi.
   * Hanya berlaku kalau cache pernah ada, artinya portal pernah terhubung. */
  if ($age > MIKHMON_HEARTBEAT_MAX_AGE) {
    $out['state'] = 'expired';
    $out['active'] = false;
    $out['stale'] = true;
    $out['message'] = 'Data langganan sudah lebih dari '
                    . (int) round(MIKHMON_HEARTBEAT_MAX_AGE / 86400)
                    . ' hari tidak diperbarui. Hubungi NOCIFY.';
  }

  $GLOBALS['mikhmon_license_status'] = $out;
  return $out;
}

/** true kalau panel harus dikunci. */
function mikhmon_license_locked() {
  $st = mikhmon_license_status();
  return ($st['state'] == 'expired');
}

/** true kalau perlu ditampilkan peringatan di atas halaman. */
function mikhmon_license_warn() {
  $st = mikhmon_license_status();
  return in_array($st['state'], array('warning', 'grace', 'expired'));
}

/** ABCD1234EFGH -> ABCD-1234-EFGH */
function mikhmon_license_pretty_id($id) {
  $id = (string) $id;
  if (strlen($id) != 12) {
    return $id;
  }
  return substr($id, 0, 4) . '-' . substr($id, 4, 4) . '-' . substr($id, 8, 4);
}

function mikhmon_wa_link($text = '') {
  $url = 'https://wa.me/' . MIKHMON_WA_NUMBER;
  if ($text !== '') {
    $url .= '?text=' . rawurlencode($text);
  }
  return $url;
}

/**
 * Pita peringatan yang tampil di atas semua halaman.
 *   active/unconfigured : tanpa pita (instalasi baru tidak boleh diteriaki)
 *   warning/grace       : pita kuning
 *   expired             : pita merah, panel dikunci
 */
function mikhmon_license_banner() {
  if (!mikhmon_license_warn()) {
    return '';
  }
  $st = mikhmon_license_status();

  if ($st['state'] == 'expired') {
    return '<div class="box bg-danger" style="margin:0;border-radius:0;text-align:center">'
         . '<i class="fa fa-lock"></i> <b>Langganan Mikhmon berakhir, panel dikunci.</b> '
         . 'Perpanjang lewat portal supaya panel bisa dipakai lagi. '
         . '<a href="./admin.php?id=subscription" style="color:#fff;text-decoration:underline">Buka halaman Langganan</a>'
         . '</div>';
  }

  if ($st['state'] == 'grace') {
    return '<div class="box bg-warning" style="margin:0;border-radius:0;text-align:center">'
         . '<i class="fa fa-exclamation-triangle"></i> <b>Langganan sudah lewat '
         . ($st['days'] !== null ? htmlspecialchars(abs((int) $st['days']), ENT_QUOTES) : 'beberapa')
         . ' hari.</b> Segera perpanjang supaya panel tidak dikunci. '
         . '<a href="./admin.php?id=subscription">Perpanjang sekarang</a>'
         . '</div>';
  }

  $text = 'Langganan Mikhmon segera berakhir';
  if ($st['days'] !== null) {
    $text .= ' (' . (int) $st['days'] . ' hari lagi)';
  }
  if ($st['expires'] != '') {
    $text .= ', berlaku sampai ' . htmlspecialchars($st['expires'], ENT_QUOTES);
  }

  return '<div class="box bg-warning" style="margin:0;border-radius:0;text-align:center">'
       . '<i class="fa fa-clock-o"></i> ' . $text . '. '
       . '<a href="./admin.php?id=subscription">Lihat langganan</a>'
       . '</div>';
}
