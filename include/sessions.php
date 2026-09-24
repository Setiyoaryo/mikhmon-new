<?php
/*
 *  Mikhmon - satu berkas per sesi router.
 *
 *  Copyright (C) 2018 Laksamadi Guko.        (Mikhmon asli)
 *  Copyright (C) 2024 NOCIFY.                (tambahan fork ini)
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation; either version 2 of the License, or
 *  (at your option) any later version.
 *
 *  Dulu semua kredensial router (IP, user, password) ada di satu berkas
 *  include/config.php, satu baris per sesi. Panel ini di-host untuk banyak
 *  pelanggan, jadi satu permintaan yang hanya butuh satu router tidak boleh
 *  membaca password router pelanggan lain.
 *
 *  Sekarang tiap sesi punya berkasnya sendiri di include/sessions/<nama>.php
 *  yang isinya satu baris $data['<nama>'] = array (...); dengan format yang
 *  sama seperti dulu, jadi include/readcfg.php tidak perlu diubah.
 *
 *  Fungsi di bawah sengaja tidak memuat kredensial kecuali diminta:
 *  mikhmon_session_names() hanya membaca daftar direktori.
 */

if (defined('MIKHMON_SESSIONS_LOADED')) {
  return;
}
define('MIKHMON_SESSIONS_LOADED', 1);

if (!defined('MIKHMON_SESSION_DIR')) {
  define('MIKHMON_SESSION_DIR', dirname(__FILE__) . '/sessions');
}

/* Berkas kunci per sesi; bisa diarahkan lain oleh tools/ di CLI. */
if (!defined('MIKHMON_SESSION_KEYFILE')) {
  define('MIKHMON_SESSION_KEYFILE', dirname(__FILE__) . '/tenantkey.php');
}

if (!function_exists('mikhmon_session_dir')) {
  function mikhmon_session_dir() {
    return rtrim(MIKHMON_SESSION_DIR, '/\\') . '/';
  }
}

/*
 * Nama sesi dipakai untuk nama berkas, kunci di config.php, dan kunci di
 * include/tenantkey.php. mikhmon_cfg_name() sudah membersihkannya, tetapi
 * berkas ini bisa dimuat sebelum readcfg.php (config.php memuatnya dulu),
 * jadi ada fallback kecil untuk kasus itu.
 */
if (!function_exists('mikhmon_session_safe')) {
  function mikhmon_session_safe($name) {
    if (!is_scalar($name)) {
      return '';
    }
    $name = (string) $name;
    if (function_exists('mikhmon_cfg_name')) {
      $name = mikhmon_cfg_name($name);
    } else {
      $name = preg_replace('/\s+/', '-', str_replace(array("'", '"', '<', '>', "\\", "\r", "\n", "\0"), '', $name));
      $name = preg_replace('/[^A-Za-z0-9._-]/', '', (string) $name);
      $name = trim((string) $name, '-');
    }
    if ($name === '' || $name === '.' || $name === '..' || $name[0] === '.') {
      return '';
    }
    return $name;
  }
}

/* Path berkas satu sesi, atau '' kalau namanya tidak masuk akal. */
if (!function_exists('mikhmon_session_file')) {
  function mikhmon_session_file($name) {
    $name = mikhmon_session_safe($name);
    if ($name === '' || $name === 'mikhmon') {
      return '';
    }
    // Cek struktur isolasi data/tenants/<nama>/session.php lebih dulu
    $tenantSession = dirname(__FILE__) . '/../data/tenants/' . strtolower($name) . '/session.php';
    if (is_file($tenantSession)) {
      return $tenantSession;
    }
    return mikhmon_session_dir() . $name . '.php';
  }
}

/*
 * Nama sesi yang tersimpan di include/sessions/. Tidak membaca kredensial.
 * Kalau direktori belum ada atau masih kosong (instalasi lama yang belum
 * dipindahkan), nama diambil dari baris $data['...'] yang masih ada di
 * include/config.php.
 */
if (!function_exists('mikhmon_session_names')) {
  function mikhmon_session_names() {
    $names = array();

    $files = @scandir(mikhmon_session_dir());
    if (is_array($files)) {
      foreach ($files as $file) {
        if (strlen($file) < 5 || substr($file, -4) !== '.php' || $file[0] === '.') {
          continue;
        }
        $name = substr($file, 0, -4);
        if (mikhmon_session_safe($name) !== $name) {
          continue;
        }
        $names[$name] = $name;
      }
    }

    // Scan juga folder terisolasi data/tenants/*/session.php
    $tenantDir = dirname(__FILE__) . '/../data/tenants';
    if (is_dir($tenantDir)) {
      $tFolders = @scandir($tenantDir);
      if (is_array($tFolders)) {
        foreach ($tFolders as $tf) {
          if ($tf === '.' || $tf === '..' || !is_dir($tenantDir . '/' . $tf)) {
            continue;
          }
          if (is_file($tenantDir . '/' . $tf . '/session.php')) {
            $name = mikhmon_session_safe($tf);
            if ($name !== '') {
              $names[$name] = $name;
            }
          }
        }
      }
    }

    foreach (mikhmon_session_legacy_names() as $name) {
      $names[$name] = $name;
    }

    $names = array_values($names);
    sort($names, SORT_STRING);
    return $names;
  }
}

/*
 * Nama sesi yang masih tersimpan sebagai baris di include/config.php.
 * Hanya dipakai sebagai cadangan supaya instalasi yang belum dipindahkan
 * tetap jalan.
 */
if (!function_exists('mikhmon_session_legacy_names')) {
  function mikhmon_session_legacy_names() {
    $names = array();
    $raw = @file_get_contents(dirname(__FILE__) . '/config.php');
    if ($raw === false) {
      return $names;
    }
    if (preg_match_all('/^[ \t]*\$data\[\'([^\']+)\'\]/m', $raw, $match)) {
      foreach ($match[1] as $name) {
        if ($name === 'mikhmon' || mikhmon_session_safe($name) !== $name) {
          continue;
        }
        $names[$name] = $name;
      }
    }
    return array_values($names);
  }
}

/* true kalau sesi ini punya berkasnya sendiri atau masih ada di config.php. */
if (!function_exists('mikhmon_session_exists')) {
  function mikhmon_session_exists($name) {
    $name = mikhmon_session_safe($name);
    if ($name === '' || $name === 'mikhmon') {
      return false;
    }
    $file = mikhmon_session_file($name);
    if ($file !== '' && is_file($file)) {
      return true;
    }
    if (isset($GLOBALS['data'][$name])) {
      return true;
    }
    return in_array($name, mikhmon_session_legacy_names(), true);
  }
}

/*
 * Di mana sesi ini disimpan: 'file' (include/sessions/), 'config' (masih
 * sebagai baris lama di include/config.php), atau '' (belum ada).
 */
if (!function_exists('mikhmon_session_storage')) {
  function mikhmon_session_storage($name) {
    $name = mikhmon_session_safe($name);
    if ($name === '' || $name === 'mikhmon') {
      return '';
    }
    $file = mikhmon_session_file($name);
    if ($file !== '' && is_file($file)) {
      return 'file';
    }
    if (in_array($name, mikhmon_session_legacy_names(), true)) {
      return 'config';
    }
    return '';
  }
}

/*
 * Muat kredensial satu sesi ke $data. Berkas yang jelas bukan berkas sesi
 * (tidak berisi baris $data['<nama>'] = array (...) tidak dimuat, dan berkas
 * yang hilang tidak pernah membuat fatal.
 */
if (!function_exists('mikhmon_session_load')) {
  function mikhmon_session_load($name) {
    $name = mikhmon_session_safe($name);
    if ($name === '' || $name === 'mikhmon') {
      return false;
    }

    if (!empty($GLOBALS['mikhmon_session_loaded'][$name])) {
      mikhmon_session_key_use($name);
      return true;
    }

    $file = mikhmon_session_file($name);
    if ($file !== '' && is_file($file)) {
      $raw = @file_get_contents($file);
      if ($raw === false || !preg_match('/^[ \t]*\$data\[\'' . preg_quote($name, '/') . '\'\]\s*=\s*array \(/m', $raw)) {
        return false;
      }
      $GLOBALS['mikhmon_session_loaded'][$name] = true;
      global $data;
      include $file;
      mikhmon_session_key_use($name);
      return isset($data[$name]);
    }

    // Instalasi lama: barisnya masih ada di include/config.php dan sudah
    // dimuat saat berkas itu di-include.
    if (isset($GLOBALS['data'][$name])) {
      $GLOBALS['mikhmon_session_loaded'][$name] = true;
      mikhmon_session_key_use($name);
      return true;
    }

    return false;
  }
}

/* Muat semua sesi. Dipakai kalau permintaan memang butuh daftarnya. */
if (!function_exists('mikhmon_session_load_all')) {
  function mikhmon_session_load_all() {
    foreach (mikhmon_session_names() as $name) {
      mikhmon_session_load($name);
    }
  }
}

/*
 * Dipanggil include/config.php (dan oleh config.php hasil migrasi) untuk
 * memuat sesi yang dipakai permintaan ini. Kalau permintaan datang dari
 * <nama>.nocify.id, hanya sesi pelanggan itu yang dimuat; kalau ada
 * ?session=<nama>, hanya sesi itu; selain itu semuanya.
 */
if (!function_exists('mikhmon_config_boot')) {
  function mikhmon_config_boot($session = '') {
    if (!function_exists('mikhmon_tenant_session') && is_file(dirname(__FILE__) . '/tenant.php')) {
      include_once(dirname(__FILE__) . '/tenant.php');
    }
    $tenant = function_exists('mikhmon_tenant_session') ? mikhmon_tenant_session() : '';

    if ($tenant !== '') {
      mikhmon_session_load($tenant);
    } elseif (isset($session) && $session !== '' && mikhmon_session_exists($session)) {
      mikhmon_session_load($session);
    } else {
      mikhmon_session_load_all();
    }
  }
}

/*
 * true kalau include/config.php sudah memakai berkas sesi (ada pemanggilan
 * mikhmon_config_boot()). Instalasi lama yang barisnya masih di config.php
 * balikannya false, jadi penulisannya tetap ke config.php seperti dulu.
 */
if (!function_exists('mikhmon_config_uses_files')) {
  function mikhmon_config_uses_files() {
    $raw = @file_get_contents(dirname(__FILE__) . '/config.php');
    if ($raw === false) {
      return true;
    }
    return strpos($raw, 'mikhmon_config_boot') !== false;
  }
}

/* Header kecil yang selalu ada di atas tiap berkas sesi. */
if (!function_exists('mikhmon_session_guard')) {
  function mikhmon_session_guard($name) {
    $base = $name . '.php';
    return "<?php\n"
      . "/* Sesi router satu pelanggan. Dibuat otomatis oleh Mikhmon, jangan di-commit. */\n"
      . "if (isset(\$_SERVER['REQUEST_URI']) && substr(\$_SERVER['REQUEST_URI'], -" . strlen($base) . ") == '" . $base . "') { header('Location:./'); };\n";
  }
}

/*
 * Simpan satu baris sesi (hasil cfg_line()) ke berkasnya sendiri.
 * Ditulis lewat mikhmon_cfg_write() (temporary file + rename) supaya tidak
 * ada pembaca yang melihat berkas setengah jadi.
 */
if (!function_exists('mikhmon_session_save')) {
  function mikhmon_session_save($name, $line) {
    $name = mikhmon_session_safe($name);
    if ($name === '' || $name === 'mikhmon') {
      return false;
    }
    $line = trim((string) $line);
    if (!preg_match('/^\$data\[\'' . preg_quote($name, '/') . '\'\]\s*=\s*array \(/', $line)) {
      return false;
    }

    $dir = rtrim(mikhmon_session_dir(), '/');
    if (!is_dir($dir)) {
      @mkdir($dir, 0775, true);
    }
    if (!is_dir($dir) || !function_exists('mikhmon_cfg_write')) {
      return false;
    }

    return mikhmon_cfg_write(mikhmon_session_file($name), mikhmon_session_guard($name) . $line . "\n");
  }
}

/*
 * Untuk instalasi yang belum dipindahkan: tulis baris sesi kembali ke
 * include/config.php, mengganti baris lama (atau menambahkannya kalau
 * $append true). Dipakai settings/settings.php selama include/sessions/
 * belum dipakai, supaya instalasi lama tetap berjalan persis seperti dulu.
 */
if (!function_exists('mikhmon_session_save_config')) {
  function mikhmon_session_save_config($name, $line, $append = false) {
    $name = mikhmon_session_safe($name);
    if ($name === '' || $name === 'mikhmon') {
      return false;
    }
    $line = trim((string) $line);
    if (!preg_match('/^\$data\[\'' . preg_quote($name, '/') . '\'\]\s*=\s*array \(/', $line)) {
      return false;
    }
    if (!function_exists('mikhmon_cfg_write')) {
      return false;
    }

    $cfg = dirname(__FILE__) . '/config.php';
    $raw = @file_get_contents($cfg);
    if ($raw === false) {
      return false;
    }
    $lines = preg_split("/\r\n|\n|\r/", $raw);
    $found = false;
    $pattern = '/^[ \t]*\$data\[\'' . preg_quote($name, '/') . '\'\]/';
    foreach ($lines as $i => $cfgline) {
      if (preg_match($pattern, $cfgline)) {
        $lines[$i] = $line;
        $found = true;
      }
    }
    if (!$found) {
      if (!$append) {
        return false;
      }
      // Baris terakhir biasanya kosong (berkas diakhiri newline); sisipkan
      // sebelum kekosongan itu supaya akhir berkas tetap rapi.
      $at = count($lines);
      while ($at > 0 && trim($lines[$at - 1]) === '') {
        $at--;
      }
      array_splice($lines, $at, 0, array($line));
    }
    return mikhmon_cfg_write($cfg, implode("\n", $lines));
  }
}

/*
 * Hapus satu sesi: berkasnya sendiri, dan baris lamanya di include/config.php
 * kalau instalasi ini belum dipindahkan. Sesi lain (dan akun admin) tidak
 * disentuh.
 */
if (!function_exists('mikhmon_session_delete')) {
  function mikhmon_session_delete($name) {
    $name = mikhmon_session_safe($name);
    if ($name === '' || $name === 'mikhmon') {
      return false;
    }

    $removed = false;
    $file = mikhmon_session_file($name);
    if ($file !== '' && is_file($file)) {
      if (@unlink($file)) {
        $removed = true;
      }
    }
    unset($GLOBALS['mikhmon_session_loaded'][$name]);
    unset($GLOBALS['data'][$name]);

    $needle = "\$data['" . $name . "']";
    $cfg = dirname(__FILE__) . '/config.php';
    $raw = @file_get_contents($cfg);
    if ($raw !== false && strpos($raw, $needle) !== false && function_exists('mikhmon_cfg_write')) {
      $keep = array();
      foreach (preg_split("/\r\n|\n|\r/", $raw) as $line) {
        if (strpos($line, $needle) !== false) {
          $removed = true;
          continue;
        }
        $keep[] = $line;
      }
      mikhmon_cfg_write($cfg, implode("\n", $keep));
    }

    return $removed;
  }
}

/*
 * Kunci enkripsi per sesi, dibaca dari include/tenantkey.php (tidak ikut
 * di-commit; contohnya ada di include/tenantkey.example.php).
 */
if (!function_exists('mikhmon_session_keys')) {
  function mikhmon_session_keys() {
    if (!isset($GLOBALS['mikhmon_session_keys']) || !is_array($GLOBALS['mikhmon_session_keys'])) {
      $GLOBALS['mikhmon_session_keys'] = array();
      $file = MIKHMON_SESSION_KEYFILE;
      if (is_file($file)) {
        $mikhmon_session_keys = array();
        include $file;
        if (isset($mikhmon_session_keys) && is_array($mikhmon_session_keys)) {
          $GLOBALS['mikhmon_session_keys'] = $mikhmon_session_keys;
        }
      }
    }
    return $GLOBALS['mikhmon_session_keys'];
  }
}

/* Kunci satu sesi, atau '' kalau sesi itu belum diberi kunci sendiri. */
if (!function_exists('mikhmon_session_key')) {
  function mikhmon_session_key($name) {
    $name = mikhmon_session_safe($name);
    if ($name === '') {
      return '';
    }
    $keys = mikhmon_session_keys();
    if (!isset($keys[$name]) || !is_scalar($keys[$name])) {
      return '';
    }
    return trim((string) $keys[$name]);
  }
}

/* Pakai kunci sesi ini untuk encrypt()/decrypt() berikutnya. */
if (!function_exists('mikhmon_session_key_use')) {
  function mikhmon_session_key_use($name) {
    $key = mikhmon_session_key($name);
    $GLOBALS['mikhmon_cfg_key'] = $key;
    return $key;
  }
}

/* Kunci sesi yang sedang dipakai permintaan ini ('' = belum ada). */
if (!function_exists('mikhmon_session_current_key')) {
  function mikhmon_session_current_key() {
    return isset($GLOBALS['mikhmon_cfg_key']) ? (string) $GLOBALS['mikhmon_cfg_key'] : '';
  }
}
