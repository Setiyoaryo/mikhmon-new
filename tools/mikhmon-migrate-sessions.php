<?php
/*
 *  Mikhmon - pindahkan sesi router dari include/config.php ke include/sessions/.
 *
 *  Dulu semua kredensial router ada di include/config.php, satu baris per
 *  sesi. Skrip ini memecahnya menjadi satu berkas per sesi supaya permintaan
 *  milik satu pelanggan tidak pernah memuat password pelanggan lain, lalu
 *  menulis ulang include/config.php hanya dengan baris akun admin.
 *
 *  Pemakaian (dari direktori utama aplikasi):
 *      php tools/mikhmon-migrate-sessions.php
 *      php tools/mikhmon-migrate-sessions.php [berkas-config] [direktori-sesi]
 *
 *  Berkas asli dicadangkan ke <berkas-config>.bak lebih dulu. Tidak ada baris
 *  yang hilang: baris $data['...'] per sesi ditulis ke berkas sesinya, baris
 *  lain (penjaga akses langsung dan $data['mikhmon']) tetap di config.php.
 */

if (PHP_SAPI !== 'cli') {
  if (!headers_sent()) {
    header('HTTP/1.1 403 Forbidden');
  }
  echo "Skrip ini hanya bisa dijalankan dari command line.\n";
  exit(1);
}

$mikhmon_root = dirname(__DIR__);
$mikhmon_cfgfile = isset($argv[1]) ? $argv[1] : $mikhmon_root . '/include/config.php';
$mikhmon_sessdir = isset($argv[2]) ? rtrim($argv[2], '/\\') : $mikhmon_root . '/include/sessions';

function mikhmon_migrate_stop($message) {
  fwrite(STDERR, "GAGAL: " . $message . "\n");
  exit(1);
}

if (!is_file($mikhmon_cfgfile)) {
  mikhmon_migrate_stop("berkas config tidak ditemukan: " . $mikhmon_cfgfile);
}
$raw = @file_get_contents($mikhmon_cfgfile);
if ($raw === false) {
  mikhmon_migrate_stop("tidak bisa membaca " . $mikhmon_cfgfile);
}

if (!is_dir($mikhmon_sessdir)) {
  @mkdir($mikhmon_sessdir, 0775, true);
}
if (!is_dir($mikhmon_sessdir)) {
  mikhmon_migrate_stop("tidak bisa membuat direktori sesi: " . $mikhmon_sessdir);
}

$backup = $mikhmon_cfgfile . '.bak';
if (@copy($mikhmon_cfgfile, $backup) === false) {
  mikhmon_migrate_stop("gagal membuat cadangan " . $backup);
}

// Kunci untuk mikhmon_session_save() (mikhmon_cfg_write + sanitasi nama).
// readcfg.php butuh $data/$session; di CLI keduanya kosong dan semua notice
// dimatikan readcfg.php sendiri.
if (!isset($_SERVER['REQUEST_URI'])) {
  $_SERVER['REQUEST_URI'] = '';
}
$data = array();
$session = '';
include $mikhmon_root . '/include/readcfg.php';
define('MIKHMON_SESSION_DIR', $mikhmon_sessdir);
include $mikhmon_root . '/include/sessions.php';
error_reporting(E_ALL);

echo "== Migrasi sesi Mikhmon ==\n";
echo "config   : " . $mikhmon_cfgfile . "\n";
echo "sesi     : " . $mikhmon_sessdir . "/\n";
echo "cadangan : " . $backup . "\n\n";

$lines = preg_split("/\r\n|\n|\r/", $raw);
$keep = array();
$sessions = array();
$duplicates = 0;

foreach ($lines as $line) {
  if (!preg_match('/^[ \t]*\$data\[\'([^\']+)\'\]/', $line, $match)) {
    $keep[] = $line;
    continue;
  }
  $name = $match[1];
  if ($name === 'mikhmon') {
    $keep[] = $line;
    continue;
  }
  if (mikhmon_session_safe($name) !== $name) {
    // Nama yang tidak aman dipakai sebagai nama berkas: biarkan di config.php
    // supaya tidak ada data yang hilang.
    $keep[] = $line;
    echo "PERINGATAN: nama sesi '" . $name . "' tidak bisa dipindahkan, baris tetap di config.php\n";
    continue;
  }
  if (isset($sessions[$name])) {
    $duplicates++;
    echo "PERINGATAN: sesi '" . $name . "' muncul lebih dari sekali, yang terakhir yang dipakai\n";
  }
  $sessions[$name] = $line;
}

if (empty($sessions)) {
  echo "Tidak ada baris sesi router di config.php. Tidak ada yang dipindahkan.\n";
  echo "config.php tidak diubah.\n";
  exit(0);
}

ksort($sessions, SORT_STRING);
$ok = 0;
foreach ($sessions as $name => $line) {
  $line = trim($line);
  $target = $mikhmon_sessdir . '/' . $name . '.php';
  $existing = is_file($target) ? ' (ditimpa)' : '';
  if (mikhmon_session_save($name, $line)) {
    $ok++;
    echo "sesi '" . $name . "' -> include/sessions/" . $name . ".php" . $existing
      . " (" . (substr_count($line, "','") + 1) . " bidang)\n";
  } else {
    // Kalau gagal, barisnya tetap di config.php: lebih baik sesi masih terbaca
    // daripada hilang tanpa jejak.
    $keep[] = $line;
    echo "GAGAL menulis include/sessions/" . $name . ".php, baris tetap di config.php\n";
  }
}
// config.php baru: baris non-$data dari berkas asli (penjaga akses langsung,
// komentar), baris akun admin, lalu pemuat sesi per pelanggan. Kredensial
// router tidak lagi ada di sini.
$newcfg = implode("\n", $keep);
if (strpos($newcfg, 'mikhmon_config_boot') === false) {
  if (!preg_match('/^[ \t]*\$data\[\'mikhmon\'\]/m', $newcfg)) {
    echo "PERINGATAN: baris akun admin tidak ditemukan, memakai akun bawaan\n";
    $newcfg = rtrim($newcfg, "\r\n") . "\n"
      . "\$data['mikhmon'] = array ('1'=>'mikhmon<|<mikhmon','mikhmon>|>aWNlbA==');";
  }
  $newcfg = rtrim($newcfg, "\r\n") . "\n\n"
    . "include_once(dirname(__FILE__) . '/sessions.php');\n"
    . "mikhmon_config_boot(isset(\$session) ? \$session : '');\n";
}
if (mikhmon_cfg_write($mikhmon_cfgfile, $newcfg) === false) {
  mikhmon_migrate_stop("gagal menulis ulang " . $mikhmon_cfgfile . " (cadangan ada di " . $backup . ")");
}

echo "\nconfig.php ditulis ulang, " . preg_match_all('/^[ \t]*\$data\[/m', $newcfg) . " baris \$data tersisa (akun admin + yang gagal dipindahkan).\n";
if ($duplicates > 0) {
  echo "Catatan: ada " . $duplicates . " baris sesi duplikat.\n";
}
echo "Selesai: " . $ok . " sesi dipindahkan.\n";
echo "Langkah berikutnya: beri kunci per sesi dan enkripsi ulang password lama:\n";
echo "    php tools/mikhmon-rekey.php --all\n";
