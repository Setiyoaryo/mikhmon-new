#!/usr/bin/env php
<?php
/*
 *  Mikhmon Nocify - daftarkan sesi router yang sudah ada ke portal langganan.
 *
 *  Copyright (C) 2024 NOCIFY.  GPLv2.
 *
 *  Dipakai sekali saat pindah ke model langganan: alih-alih mengetik ulang
 *  setiap pelanggan di halaman admin portal, alat ini membaca sesi router yang
 *  sudah ada di panel (include/sessions/*.php, atau include/config.php pada
 *  instalasi yang belum dipindahkan) lalu membuatkan satu pelanggan di portal
 *  untuk masing-masing sesi.
 *
 *  Cara pakai:
 *
 *    php tools/mikhmon-import-to-portal.php --portal=https://control.nocify.id \
 *        --password=KATASANDI_ADMIN --instance=ID_PANEL
 *
 *    tambahan:
 *      --until=YYYY-MM-DD   masa berlaku untuk semua pelanggan yang diimpor
 *                           (bawaan: 30 hari dari hari ini, supaya tidak ada
 *                           panel yang terkunci pada hari pindah)
 *      --dry-run            tampilkan rencananya saja, tidak mengirim apa pun
 *
 *  ID panel didapat dari halaman admin portal, kartu "Panel terpasang".
 *  Jalankan ulang alat ini dengan aman: pelanggan yang sudah ada akan
 *  dilaporkan sebagai "sudah ada", bukan dibuat dobel.
 */

if (PHP_SAPI !== 'cli') {
  header('HTTP/1.1 403 Forbidden');
  echo "Alat ini hanya bisa dijalankan dari baris perintah.\n";
  exit(1);
}

$root = dirname(dirname(__FILE__));
$opt = array();
foreach (array_slice($argv, 1) as $arg) {
  if (substr($arg, 0, 2) === '--') {
    $kv = explode('=', substr($arg, 2), 2);
    $opt[$kv[0]] = isset($kv[1]) ? $kv[1] : true;
  }
}

function imp_usage() {
  echo "Mikhmon Nocify - impor pelanggan dari panel ke portal\n\n";
  echo "  --portal=URL         alamat portal, mis. https://control.nocify.id\n";
  echo "  --password=SANDI     kata sandi admin portal\n";
  echo "  --instance=ID        id panel di portal (kartu Panel terpasang)\n";
  echo "  --until=YYYY-MM-DD   masa berlaku untuk semua yang diimpor\n";
  echo "  --dry-run            hanya menampilkan rencananya\n";
  echo "  --config=BERKAS      bawaan: include/config.php\n";
  echo "  --sessions-dir=DIR   bawaan: include/sessions\n";
}

if (isset($opt['help']) || isset($opt['h'])) {
  imp_usage();
  exit(0);
}
foreach (array('portal', 'password', 'instance') as $wajib) {
  if (!isset($opt[$wajib]) || $opt[$wajib] === true || trim($opt[$wajib]) === '') {
    echo "Wajib: --$wajib\n\n";
    imp_usage();
    exit(1);
  }
}

$portal   = rtrim($opt['portal'], '/');
$password = $opt['password'];
$instance = trim($opt['instance']);
$dry      = isset($opt['dry-run']);
$config   = isset($opt['config']) ? $opt['config'] : $root . '/include/config.php';
$sessDir  = isset($opt['sessions-dir']) ? rtrim($opt['sessions-dir'], '/') : $root . '/include/sessions';

$until = isset($opt['until']) && $opt['until'] !== true
  ? $opt['until']
  : date('Y-m-d', strtotime('+30 days'));
if (!preg_match('/^\d{4}-\d{2}-\d{2}$/', $until)) {
  echo "Tanggal --until harus berbentuk YYYY-MM-DD.\n";
  exit(1);
}

/* ------------------------------------------------------- baca sesi panel */

/*
 * Setiap berkas sesi berisi satu baris $data['<nama>'] dengan 11 bidang yang
 * dipisah tanda khusus. Yang dibutuhkan di sini cuma nama sesi dan nama
 * hotspotnya, jadi pembacaannya sederhana.
 */
function imp_baca_sesi($berkas) {
  $hasil = array();
  $isi = @file_get_contents($berkas);
  if ($isi === false) {
    return $hasil;
  }
  foreach (preg_split('/\r\n|\n/', $isi) as $baris) {
    if (strpos($baris, '$data[') !== 0) {
      continue;
    }
    if (preg_match("/^\\\$data\\['([^']+)'\\]/", $baris, $m) !== 1) {
      continue;
    }
    $nama = $m[1];
    if ($nama === 'mikhmon') {
      continue; // akun admin, bukan sesi router
    }
    // Bidang ke-4 berbentuk 'sesi%NamaHotspot'. Tanda persen pertama di baris
    // itu adalah pemisahnya, karena nama sesi maupun password (base64) tidak
    // pernah mengandung persen.
    $hotspot = '';
    if (preg_match("/%([^']*)/", $baris, $h) === 1) {
      $hotspot = $h[1];
    }
    $hasil[$nama] = array('nama' => $nama, 'hotspot' => $hotspot);
  }
  return $hasil;
}

$sesi = array();
if (is_dir($sessDir)) {
  foreach (glob($sessDir . '/*.php') as $f) {
    $sesi += imp_baca_sesi($f);
  }
}
if (!$sesi && is_file($config)) {
  $sesi = imp_baca_sesi($config);
}

if (!$sesi) {
  echo "Tidak ada sesi router yang ditemukan di:\n";
  echo "  $sessDir\n";
  echo "  $config\n";
  echo "\nPastikan alat ini dijalankan dari folder utama Mikhmon.\n";
  exit(1);
}

/* ------------------------------------------------------------- ke portal */

/*
 * Satu permintaan HTTP ke portal. Memakai cURL kalau ada, kalau tidak jatuh ke
 * pembungkus stream - sama seperti mikhmon_api_post() di
 * lib/routeros_api.class.php. Balikannya: [kode, badan, header].
 */
function imp_http($method, $url, $body = null, $cookie = '') {
  $payload = $body === null ? null : json_encode($body);

  if (function_exists('curl_init')) {
    $ch = curl_init($url);
    curl_setopt_array($ch, array(
      CURLOPT_RETURNTRANSFER => true,
      CURLOPT_HEADER => true,
      CURLOPT_TIMEOUT => 30,
      CURLOPT_CUSTOMREQUEST => $method,
      CURLOPT_HTTPHEADER => array('Content-Type: application/json'),
    ));
    if ($payload !== null) {
      curl_setopt($ch, CURLOPT_POSTFIELDS, $payload);
    }
    if ($cookie !== '') {
      curl_setopt($ch, CURLOPT_COOKIE, $cookie);
    }
    $raw = (string) curl_exec($ch);
    $kode = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $panjang = (int) curl_getinfo($ch, CURLINFO_HEADER_SIZE);
    curl_close($ch);
    return array($kode, substr($raw, $panjang), substr($raw, 0, $panjang));
  }

  $headers = "Content-Type: application/json\r\n";
  if ($cookie !== '') {
    $headers .= 'Cookie: ' . $cookie . "\r\n";
  }
  $ctx = stream_context_create(array('http' => array(
    'method' => $method,
    'header' => $headers,
    'content' => $payload === null ? '' : $payload,
    'timeout' => 30,
    'ignore_errors' => true,
  )));
  $badan = @file_get_contents($url, false, $ctx);
  $kode = 0;
  $kepala = '';
  if (isset($http_response_header) && is_array($http_response_header)) {
    $kepala = implode("\r\n", $http_response_header);
    if (preg_match('#^HTTP/\S+\s+(\d+)#', $http_response_header[0], $m) === 1) {
      $kode = (int) $m[1];
    }
  }
  return array($kode, $badan === false ? '' : $badan, $kepala);
}

echo "Portal   : $portal\n";
echo "Panel    : $instance\n";
echo "Berlaku sampai: $until\n";
echo "Sesi     : " . count($sesi) . " ditemukan\n";
echo str_repeat('-', 60) . "\n";

if ($dry) {
  foreach ($sesi as $s) {
    $slug = strtolower(preg_replace('/[^A-Za-z0-9]+/', '-', $s['nama']));
    $slug = trim($slug, '-');
    printf("  %-20s -> pelanggan \"%s\", subdomain %s.nocify.id\n",
      $s['nama'], $s['hotspot'] !== '' ? $s['hotspot'] : $s['nama'], $slug);
  }
  echo str_repeat('-', 60) . "\n";
  echo "Ini baru rencana. Jalankan tanpa --dry-run untuk mengirim.\n";
  exit(0);
}

// Masuk sebagai admin portal, lalu simpan cookie sesinya.
list($kode, $badan, $kepala) = imp_http(
  'POST', $portal . '/api/v1/admin/login', array('password' => $password));

if ($kode !== 204) {
  echo "Gagal masuk ke portal (HTTP $kode). Periksa --portal dan --password.\n";
  exit(1);
}

$cookie = '';
if (preg_match('/^Set-Cookie:\s*([^;]+)/mi', $kepala, $m) === 1) {
  $cookie = trim($m[1]);
}
if ($cookie === '') {
  echo "Portal tidak mengirim cookie sesi. Periksa konfigurasi portalnya.\n";
  exit(1);
}

$baru = 0; $ada = 0; $gagal = 0; $peringatan = 0;

foreach ($sesi as $s) {
  $namaSesi = $s['nama'];
  $slug = strtolower(preg_replace('/[^A-Za-z0-9]+/', '-', $namaSesi));
  $slug = trim($slug, '-');
  $institusi = $s['hotspot'] !== '' ? $s['hotspot'] : $namaSesi;

  if ($slug !== $namaSesi) {
    echo "  PERHATIAN  sesi \"$namaSesi\" bukan huruf kecil semua.\n";
    echo "             Subdomain hanya bisa huruf kecil, jadi sesi di panel\n";
    echo "             harus diganti nama menjadi \"$slug\" supaya cocok.\n";
    $peringatan++;
  }

  list($kode, $balasan) = imp_http('POST', $portal . '/api/v1/admin/customers', array(
    'name'         => $institusi,
    'institution'  => $institusi,
    'wa'           => '',
    'session_name' => $slug,
    'instance_id'  => $instance,
    'valid_until'  => $until,
  ), $cookie);

  $json = json_decode((string) $balasan, true);

  if ($kode === 201) {
    printf("  dibuat     %-20s %s\n", $namaSesi, isset($json['customer']['pay_url']) ? $json['customer']['pay_url'] : '');
    $baru++;
  } elseif ($kode === 400 && isset($json['message'])) {
    printf("  sudah ada  %-20s %s\n", $namaSesi, $json['message']);
    $ada++;
  } else {
    printf("  GAGAL      %-20s HTTP %s %s\n", $namaSesi, $kode,
      isset($json['message']) ? $json['message'] : (string) $balasan);
    $gagal++;
  }
}

echo str_repeat('-', 60) . "\n";
echo "Selesai: $baru dibuat, $ada sudah ada, $gagal gagal";
if ($peringatan > 0) {
  echo ", $peringatan perlu diganti nama";
}
echo "\n\n";
echo "Pelanggan yang baru dibuat langsung aktif sampai $until lewat parameter\n";
echo "--until, supaya tidak ada panel yang terkunci pada hari pindah. Sesuaikan\n";
echo "tanggal masing-masing di halaman admin portal kalau perlu.\n";
if ($peringatan > 0) {
  echo "\nJangan lupa ganti nama sesi di panel (settings/settings.php) untuk yang\n";
  echo "disebut di atas, lalu jalankan alat ini sekali lagi.\n";
}
