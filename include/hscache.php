<?php
/*
 *  Copyright (C) 2018 Laksamadi Guko.
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
 *  You should have received a copy of the GNU General Public License
 *  along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

/*
 * Simpanan sementara daftar user hotspot.
 *
 * Kenapa perlu: di satu pemasangan yang sudah jalan lama, daftar user hotspot
 * bisa berisi puluhan ribu baris - 19.396 baris di salah satu pemasangan
 * pelanggan. Mengambil seluruh daftar itu dari router memakan beberapa detik
 * (yang menyeberang bukan hanya jawabannya, tapi juga waktu router menyusunnya),
 * sedangkan halaman daftar user memintanya setiap kali dibuka: tiap pindah
 * halaman, tiap ganti filter profile, tiap ganti filter comment.
 *
 * Datanya tidak berubah cepat. Jadi hasilnya disimpan sebentar di berkas, dan
 * halaman melayaninya dari situ. Yang membuat ini tetap aman:
 *   - setiap perubahan user dari panel ini membuang simpanannya
 *     (lihat index.php), jadi yang baru dihapus tidak pernah tampil lagi;
 *   - hasil kosong TIDAK disimpan, supaya router yang sedang tidak bisa
 *     dihubungi tidak berubah jadi "tidak ada user" selama masa simpan;
 *   - masa simpannya pendek dan bisa diatur lewat MIKHMON_HSCACHE_TTL.
 *
 * Isinya ditaruh di direktori sementara (di luar direktori web) karena berisi
 * daftar user pelanggan - tidak ada gunanya bisa diunduh dari luar.
 */

if (!defined('MIKHMON_HSCACHE_TTL')) {
  $mikhmon_hscache_ttl = getenv('MIKHMON_HSCACHE_TTL');
  if ($mikhmon_hscache_ttl === false || $mikhmon_hscache_ttl === '' || !is_numeric($mikhmon_hscache_ttl)) {
    $mikhmon_hscache_ttl = 300;
  }
  define('MIKHMON_HSCACHE_TTL', max(0, (int) $mikhmon_hscache_ttl));
}

if (!function_exists('mikhmon_hscache_dir')) {
  function mikhmon_hscache_dir()
  {
    $dir = getenv('MIKHMON_HSCACHE_DIR');
    if (!is_string($dir) || trim($dir) === '') {
      $dir = sys_get_temp_dir() . '/mikhmon-hscache';
    }
    if (!is_dir($dir)) {
      @mkdir($dir, 0777, true);
    }
    return rtrim($dir, '/');
  }
}

if (!function_exists('mikhmon_hscache_file')) {
  // Kunci disaring supaya nilai dari luar tidak bisa menunjuk berkas lain.
  function mikhmon_hscache_file($kunci)
  {
    return mikhmon_hscache_dir() . '/' . preg_replace('/[^A-Za-z0-9._-]/', '', (string) $kunci) . '.ser';
  }
}

if (!function_exists('mikhmon_hscache_read')) {
  /*
   * Membaca simpanan. $umurMaks dalam detik: lewat dari itu dianggap tidak ada.
   * Berkas yang rusak atau bukan hasil serialize dianggap tidak ada juga, jadi
   * simpanan yang rusak tidak pernah bisa membuat halaman gagal.
   */
  function mikhmon_hscache_read($kunci, $umurMaks = null)
  {
    $file = mikhmon_hscache_file($kunci);
    if ($umurMaks !== null && (time() - @filemtime($file)) > $umurMaks) {
      return null;
    }
    $isi = @file_get_contents($file);
    if ($isi === false || $isi === '') {
      return null;
    }
    $data = @unserialize($isi);
    return is_array($data) ? $data : null;
  }
}

if (!function_exists('mikhmon_hscache_write')) {
  /*
   * Menulis lewat berkas sementara lalu diganti namanya, sama seperti penulisan
   * config: pembaca lain hanya melihat isi yang lama atau yang baru, tidak
   * pernah setengah jadi - dan dua permintaan bersamaan tidak saling menimpa
   * sebagian.
   */
  function mikhmon_hscache_write($kunci, $data)
  {
    $file = mikhmon_hscache_file($kunci);
    $tmp = $file . '.' . getmypid() . '.' . mt_rand(100000, 999999) . '.tmp';
    if (@file_put_contents($tmp, serialize($data)) === false) {
      return false;
    }
    if (@rename($tmp, $file)) {
      return true;
    }
    @unlink($tmp);
    return false;
  }
}

if (!function_exists('mikhmon_hscache_clear')) {
  /*
   * Membuang seluruh simpanan. Dipanggil setiap kali panel ini mengubah data
   * user hotspot, supaya perubahannya langsung terlihat dan tidak menunggu masa
   * simpan habis.
   */
  function mikhmon_hscache_clear()
  {
    $daftar = @glob(mikhmon_hscache_dir() . '/*.ser');
    foreach ((array) $daftar as $file) {
      @unlink($file);
    }
    foreach ((array) @glob(mikhmon_hscache_dir() . '/*.tmp') as $file) {
      @unlink($file);
    }
  }
}

if (!function_exists('mikhmon_hscache_hotspot_users')) {
  /*
   * Seluruh user hotspot, dari simpanan kalau masih segar.
   *
   * Sengaja selalu mengambil daftar penuh (tanpa ?profile/?comment ke router):
   * satu simpanan yang sama lalu dipakai untuk semua filter dan semua halaman,
   * jadi berpindah halaman atau berganti filter tidak lagi menghubungi router.
   */
  function mikhmon_hscache_hotspot_users($API, $sesi)
  {
    $kunci = 'hotspot-users-' . $sesi;

    $simpan = mikhmon_hscache_read($kunci, MIKHMON_HSCACHE_TTL);
    if ($simpan !== null) {
      return $simpan;
    }

    if (!is_object($API)) {
      return array();
    }

    $hasil = $API->comm('/ip/hotspot/user/print');
    if (!is_array($hasil)) {
      $hasil = array();
    }

    // Hasil kosong tidak disimpan: itu bisa berarti routernya sedang tidak bisa
    // dihubungi, dan menyimpannya akan membuat panel menampilkan "tidak ada
    // user" sampai masa simpan habis.
    if (count($hasil) > 0) {
      mikhmon_hscache_write($kunci, $hasil);
    }

    return $hasil;
  }
}
