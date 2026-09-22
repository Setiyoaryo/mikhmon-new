<?php
/*
 *  Mikhmon - beri kunci per sesi dan enkripsi ulang password router.
 *
 *  Password router lama disimpan dengan kunci yang sama di semua instalasi
 *  (konstanta 128), jadi siapa pun yang punya berkasnya bisa membacanya.
 *  Skrip ini:
 *    1. membuat kunci acak (CSPRNG, 64 karakter hex) untuk sesi yang belum
 *       punya kunci dan menyimpannya di include/tenantkey.php,
 *    2. membaca password lama, lalu menuliskannya kembali dengan AES-256-CBC
 *       + HMAC memakai kunci sesi itu (format "v2:").
 *
 *  Pemakaian (dari direktori utama aplikasi):
 *      php tools/mikhmon-rekey.php --all
 *      php tools/mikhmon-rekey.php <nama-sesi>
 *
 *  Opsi:
 *      --config=BERKAS        (default include/config.php)
 *      --sessions-dir=DIR     (default include/sessions)
 *      --keyfile=BERKAS       (default include/tenantkey.php)
 *
 *  Berkas sesi (dan config.php untuk instalasi yang belum dipindahkan)
 *  dicadangkan ke <berkas>.bak sebelum ditulis. Nilai v2 yang tidak bisa
 *  dibaca dengan kunci sekarang tidak pernah ditimpa.
 */

if (PHP_SAPI !== 'cli') {
  if (!headers_sent()) {
    header('HTTP/1.1 403 Forbidden');
  }
  echo "Skrip ini hanya bisa dijalankan dari command line.\n";
  exit(1);
}

$mikhmon_root = dirname(__DIR__);
$cfgfile = $mikhmon_root . '/include/config.php';
$sessdir = $mikhmon_root . '/include/sessions';
$keyfile = $mikhmon_root . '/include/tenantkey.php';
$target = '';
$all = false;

function mikhmon_rekey_usage() {
  echo "Pemakaian: php tools/mikhmon-rekey.php [--all | <nama-sesi>] [opsi]\n";
  echo "  --config=BERKAS       default include/config.php\n";
  echo "  --sessions-dir=DIR    default include/sessions\n";
  echo "  --keyfile=BERKAS      default include/tenantkey.php\n";
}

function mikhmon_rekey_stop($message) {
  fwrite(STDERR, "GAGAL: " . $message . "\n");
  exit(1);
}

foreach (array_slice($argv, 1) as $arg) {
  if ($arg === '--all') {
    $all = true;
  } elseif ($arg === '-h' || $arg === '--help') {
    mikhmon_rekey_usage();
    exit(0);
  } elseif (strpos($arg, '--config=') === 0) {
    $cfgfile = substr($arg, 9);
  } elseif (strpos($arg, '--sessions-dir=') === 0) {
    $sessdir = rtrim(substr($arg, 15), '/\\');
  } elseif (strpos($arg, '--keyfile=') === 0) {
    $keyfile = substr($arg, 10);
  } elseif ($arg !== '' && $target === '') {
    $target = $arg;
  } else {
    fwrite(STDERR, "Argumen tidak dikenal: " . $arg . "\n");
    mikhmon_rekey_usage();
    exit(1);
  }
}

if (!$all && $target === '') {
  mikhmon_rekey_usage();
  exit(1);
}

// Konteks minimal untuk readcfg.php (penyedia mikhmon_cfg_write()), seperti di
// skrip migrasi. Semua notice dimatikan readcfg.php sendiri.
if (!isset($_SERVER['REQUEST_URI'])) {
  $_SERVER['REQUEST_URI'] = '';
}
$data = array();
$session = '';
include $mikhmon_root . '/include/readcfg.php';
define('MIKHMON_SESSION_DIR', $sessdir);
define('MIKHMON_SESSION_KEYFILE', $keyfile);
include $mikhmon_root . '/include/sessions.php';
include $mikhmon_root . '/lib/routeros_api.class.php';
error_reporting(E_ALL);

/* Baris $data['<nama>'] = array (...); dari berkas sesi atau config.php. */
function mikhmon_rekey_line($name, $cfgfile) {
  $file = mikhmon_session_file($name);
  if ($file !== '' && is_file($file)) {
    $raw = @file_get_contents($file);
    if ($raw !== false && preg_match('/^[ \t]*(\$data\[\'' . preg_quote($name, '/') . '\'\][^\r\n]*)/m', $raw, $match)) {
      return rtrim($match[1]);
    }
  }
  $raw = @file_get_contents($cfgfile);
  if ($raw !== false && preg_match('/^[ \t]*(\$data\[\'' . preg_quote($name, '/') . '\'\][^\r\n]*)/m', $raw, $match)) {
    return rtrim($match[1]);
  }
  return '';
}

if ($all) {
  $names = mikhmon_session_names();
} else {
  $names = array(mikhmon_session_safe($target));
}

if (empty($names) || $names === array('')) {
  echo "Tidak ada sesi yang bisa diproses.\n";
  exit(0);
}

echo "== Rekey sesi Mikhmon ==\n";
echo "config   : " . $cfgfile . "\n";
echo "sesi     : " . mikhmon_session_dir() . "\n";
echo "kunci    : " . $keyfile . "\n\n";

// Pastikan daftar kunci termuat, lalu ubah langsung di variabel global yang
// sama supaya mikhmon_session_key_use() ikut melihat kunci baru.
mikhmon_session_keys();
$generated = 0;
foreach ($names as $name) {
  if (!is_string($name) || $name === '') {
    echo "PERINGATAN: nama sesi tidak valid, dilewati\n";
    continue;
  }

  $line = mikhmon_rekey_line($name, $cfgfile);
  if ($line === '') {
    echo "sesi '" . $name . "': baris data tidak ditemukan, dilewati\n";
    continue;
  }
  if (!preg_match('/' . preg_quote($name . '#|#', '/') . '([^\']*)/', $line, $match)) {
    echo "sesi '" . $name . "': field password tidak ditemukan, dilewati\n";
    continue;
  }
  $cipher = $match[1];
  if ($cipher === '') {
    echo "sesi '" . $name . "': password kosong, dilewati\n";
    continue;
  }

  $newkey = false;
  if (mikhmon_session_key($name) === '') {
    $newkey = bin2hex(mikhmon_crypto_random(32));
    $GLOBALS['mikhmon_session_keys'][$name] = $newkey;
    $generated++;
  }
  mikhmon_session_key_use($name);

  if (substr($cipher, 0, 3) === 'v2:') {
    $plain = decrypt($cipher);
    if ($plain === '') {
      echo "sesi '" . $name . "': nilai v2 tidak bisa dibaca dengan kunci sekarang, "
        . "tidak ditimpa (periksa " . $keyfile . ")\n";
      continue;
    }
  } else {
    // Nilai lama selalu memakai kunci lama 128.
    $plain = decrypt($cipher, 128);
  }

  $newcipher = encrypt($plain);
  if ($newcipher === '' || substr($newcipher, 0, 3) !== 'v2:') {
    echo "sesi '" . $name . "': gagal enkripsi ulang, dilewati\n";
    continue;
  }

  $newline = str_replace($name . '#|#' . $cipher, $name . '#|#' . $newcipher, $line);
  if (strpos($newline, $newcipher) === false) {
    echo "sesi '" . $name . "': baris tidak bisa ditulis ulang, dilewati\n";
    continue;
  }

  $sessfile = mikhmon_session_file($name);
  if ($sessfile !== '' && is_file($sessfile)) {
    if (@copy($sessfile, $sessfile . '.bak') === false) {
      echo "sesi '" . $name . "': gagal membuat cadangan, tidak ditulis\n";
      continue;
    }
    if (!mikhmon_session_save($name, $newline)) {
      echo "sesi '" . $name . "': gagal menulis berkas sesi\n";
      continue;
    }
    echo "sesi '" . $name . "': include/sessions/" . $name . ".php -> v2"
      . ($newkey !== false ? ' (kunci baru)' : '') . " (cadangan: " . $name . ".php.bak)\n";
  } else {
    // Instalasi yang belum dipindahkan: barisnya masih di config.php.
    $raw = @file_get_contents($cfgfile);
    if ($raw === false || strpos($raw, $line) === false) {
      echo "sesi '" . $name . "': baris lama tidak ditemukan di config.php\n";
      continue;
    }
    if (@copy($cfgfile, $cfgfile . '.bak') === false) {
      echo "sesi '" . $name . "': gagal membuat cadangan config.php, tidak ditulis\n";
      continue;
    }
    if (mikhmon_cfg_write($cfgfile, str_replace($line, $newline, $raw)) === false) {
      echo "sesi '" . $name . "': gagal menulis config.php\n";
      continue;
    }
    echo "sesi '" . $name . "': baris lama di config.php -> v2"
      . ($newkey !== false ? ' (kunci baru)' : '') . " (cadangan: config.php.bak)\n";
  }
}

// Tulis kunci baru ke tenantkey.php (cadangkan yang lama dulu). Kalau semua
// sesi sudah punya kunci, berkasnya tidak disentuh.
$keys = mikhmon_session_keys();
if ($generated > 0) {
  $keydir = dirname($keyfile);
  if (!is_dir($keydir)) {
    @mkdir($keydir, 0775, true);
  }
  if (!is_dir($keydir)) {
    mikhmon_rekey_stop("tidak bisa membuat direktori kunci: " . $keydir);
  }
  if (is_file($keyfile) && @copy($keyfile, $keyfile . '.bak') === false) {
    mikhmon_rekey_stop("gagal membuat cadangan " . $keyfile . ".bak");
  }
  $content = "<?php\n"
    . "/*\n"
    . " * Kunci enkripsi per sesi router. Dibuat otomatis oleh\n"
    . " * tools/mikhmon-rekey.php. Jangan di-commit (lihat .gitignore);\n"
    . " * contoh dan penjelasannya ada di include/tenantkey.example.php.\n"
    . " */\n"
    . "if (isset(\$_SERVER['REQUEST_URI']) && substr(\$_SERVER['REQUEST_URI'], -14) == 'tenantkey.php') { header('Location:./'); }\n"
    . "\n"
    . "\$mikhmon_session_keys = array(\n";
  foreach ($keys as $name => $key) {
    if (mikhmon_session_safe((string) $name) !== (string) $name || !is_string($key)) {
      continue;
    }
    $content .= "  '" . $name . "' => '" . $key . "',\n";
  }
  $content .= ");\n";
  if (mikhmon_cfg_write($keyfile, $content) === false) {
    mikhmon_rekey_stop("gagal menulis " . $keyfile);
  }
  echo "\nKunci ditulis ke " . $keyfile . " untuk " . count($keys) . " sesi (cadangan: " . $keyfile . ".bak)\n";
}

echo "Selesai.\n";
