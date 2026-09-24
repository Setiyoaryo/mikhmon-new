<?php
/*
 *  Mikhmon Nocify - pemetaan subdomain ke sesi router.
 *
 *  Copyright (C) 2018 Laksamadi Guko.        (Mikhmon asli)
 *  Copyright (C) 2024 NOCIFY.                (tambahan fork ini)
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation; either version 2 of the License, or
 *  (at your option) any later version.
 *
 *  Panel ini di-host di VPS NOCIFY, dan setiap pelanggan mendapat
 *  <nama>.nocify.id. Subdomain itulah yang menentukan sesi router mana yang
 *  dipakai, sehingga pelanggan tidak perlu (dan tidak bisa) memilih sesi
 *  pelanggan lain lewat ?session=.
 */

if (defined('MIKHMON_TENANT_LOADED')) {
  return;
}
define('MIKHMON_TENANT_LOADED', 1);

/* Domain induk tempat panel di-host. */
if (!defined('MIKHMON_TENANT_DOMAIN')) {
  define('MIKHMON_TENANT_DOMAIN', 'nocify.id');
}

/* Subdomain yang bukan milik pelanggan. */
if (!defined('MIKHMON_TENANT_RESERVED')) {
  define('MIKHMON_TENANT_RESERVED', 'control,www,api,portal,panel');
}

/* Nama host yang diminta, tanpa port, huruf kecil. */
function mikhmon_tenant_host() {
  $host = isset($_SERVER['HTTP_HOST']) ? $_SERVER['HTTP_HOST'] : '';
  $host = strtolower(trim($host));
  $colon = strpos($host, ':');
  if ($colon !== false) {
    $host = substr($host, 0, $colon);
  }
  return $host;
}

/*
 * Nama sesi yang dipaksa oleh subdomain.
 * Balikannya string kosong kalau permintaan ini bukan dari subdomain
 * pelanggan - misalnya saat dibuka lewat IP, localhost, atau control.nocify.id.
 */
function mikhmon_tenant_session() {
  $host = mikhmon_tenant_host();
  if ($host === '') {
    return '';
  }
  $suffix = '.' . strtolower(MIKHMON_TENANT_DOMAIN);
  if (substr($host, -strlen($suffix)) !== $suffix) {
    return '';
  }
  $sub = substr($host, 0, strlen($host) - strlen($suffix));
  // Hanya satu label: taufiq.nocify.id, bukan a.b.nocify.id
  if ($sub === '' || strpos($sub, '.') !== false) {
    return '';
  }
  foreach (explode(',', MIKHMON_TENANT_RESERVED) as $reserved) {
    if ($sub === trim(strtolower($reserved))) {
      return '';
    }
  }
  return $sub;
}

/*
 * Panggil ini setelah session_start() dan setelah $session dari $_GET dibaca.
 * Kalau permintaannya datang dari subdomain pelanggan, sesinya dikunci ke situ
 * sehingga ?session= tidak bisa dipakai untuk mengintip sesi lain.
 */
function mikhmon_tenant_pin($session) {
  $tenant = mikhmon_tenant_session();
  if ($tenant === '') {
    $_SESSION['mikhmon_tenant'] = '';
    return $session;
  }
  $_SESSION['mikhmon_tenant'] = $tenant;
  return $tenant;
}

/* true kalau halaman ini sedang dikunci ke satu sesi oleh subdomain. */
function mikhmon_tenant_locked() {
  return isset($_SESSION['mikhmon_tenant']) && $_SESSION['mikhmon_tenant'] !== '';
}

/*
 * Direktori penyimpanan terisolasi per-tenant di data/tenants/<nama>/.
 * Berisi kustomisasi seperti session.php, login.php, logo.png, brand.txt,
 * dan hotspot captive portal files.
 */
function mikhmon_tenant_dir($tenant = '') {
  if ($tenant === '') {
    $tenant = mikhmon_tenant_session();
  }
  if ($tenant === '') {
    return '';
  }
  $clean = preg_replace('/[^a-z0-9_-]/', '', strtolower($tenant));
  if ($clean === '') {
    return '';
  }
  return dirname(__FILE__) . '/../data/tenants/' . $clean;
}

/* Mencari berkas kustom milik tenant (misal: login.php, logo.png, template.php) */
function mikhmon_tenant_file($path, $tenant = '') {
  $dir = mikhmon_tenant_dir($tenant);
  if ($dir === '') {
    return '';
  }
  $cleanPath = ltrim(preg_replace('/\.\.+/', '', (string) $path), '/');
  $file = $dir . '/' . $cleanPath;
  return is_file($file) ? $file : '';
}

/* Kredensial admin khusus tenant jika dikonfigurasi di data/tenants/<nama>/admin.php */
function mikhmon_tenant_admin($tenant = '') {
  $file = mikhmon_tenant_file('admin.php', $tenant);
  if ($file === '') {
    return null;
  }
  $t_data = array();
  include($file);
  $t = ($tenant !== '') ? $tenant : mikhmon_tenant_session();
  if (isset($t_data[$t][1], $t_data[$t][2])) {
    $u = explode('<|<', $t_data[$t][1])[1];
    $p = explode('>|>', $t_data[$t][2])[1];
    return array('user' => $u, 'pass' => $p);
  }
  return null;
}
