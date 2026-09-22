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
 * Template voucher: mana yang dipakai, dan di mana hasil suntingan disimpan.
 *
 * voucher/template.php, template-small.php, dan template-thermal.php ada di
 * dalam git - itu template bawaan. Kalau halaman Template Editor menyimpan ke
 * berkas itu juga, setiap "git pull" akan menimpanya: template yang sudah
 * disesuaikan kembali ke bawaan tanpa pemberitahuan. Itu yang dulu terjadi.
 *
 * Karena itu hasil suntingan disimpan di data/templates/ - folder yang tidak
 * ikut git, di sebelah basis data portal - dan berkas di voucher/ tetap jadi
 * bawaan. Pembacanya (cetak voucher dan pratinjau) memakai fungsi di sini supaya
 * selalu mengambil yang tersimpan kalau ada, dan bawaan kalau belum pernah
 * disunting.
 */

if (!defined('MIKHMON_VOUCHER_TEMPLATE_LOADED')) {
  define('MIKHMON_VOUCHER_TEMPLATE_LOADED', 1);

  /* Nama template yang boleh disunting. Sengaja daftar tertutup: nama ini
   * dipakai untuk menyusun nama berkas, jadi jangan sampai bisa datang dari
   * luar daftar ini. */
  if (!function_exists('mikhmon_voucher_templates')) {
    function mikhmon_voucher_templates()
    {
      return array('template', 'template-small', 'template-thermal');
    }
  }

  if (!function_exists('mikhmon_voucher_template_safe')) {
    function mikhmon_voucher_template_safe($nama)
    {
      $nama = (string) $nama;
      return in_array($nama, mikhmon_voucher_templates(), true) ? $nama : '';
    }
  }

  /* Folder hasil suntingan. Dibuat kalau belum ada, dan sengaja di luar
   * voucher/ supaya tidak pernah tertukar dengan bawaan. */
  if (!function_exists('mikhmon_voucher_template_dir')) {
    function mikhmon_voucher_template_dir()
    {
      $dir = dirname(__FILE__) . '/../data/templates';
      if (!is_dir($dir)) {
        @mkdir($dir, 0777, true);
      }
      return $dir;
    }
  }

  /* Berkas hasil suntingan untuk satu template. */
  if (!function_exists('mikhmon_voucher_template_saved')) {
    function mikhmon_voucher_template_saved($nama)
    {
      $nama = mikhmon_voucher_template_safe($nama);
      if ($nama === '') {
        return '';
      }
      return mikhmon_voucher_template_dir() . '/' . $nama . '.php';
    }
  }

  /* Berkas bawaan dari repo. */
  if (!function_exists('mikhmon_voucher_template_default')) {
    function mikhmon_voucher_template_default($nama)
    {
      $nama = mikhmon_voucher_template_safe($nama);
      if ($nama === '') {
        return '';
      }
      return dirname(__FILE__) . '/../voucher/' . $nama . '.php';
    }
  }

  /*
   * Berkas yang harus dibaca: hasil suntingan kalau ada, kalau tidak bawaan.
   * Balikannya alamat lengkap, atau string kosong kalau namanya tidak dikenal.
   */
  if (!function_exists('mikhmon_voucher_template_path')) {
    function mikhmon_voucher_template_path($nama)
    {
      $nama = mikhmon_voucher_template_safe($nama);
      if ($nama === '') {
        return '';
      }
      $tersimpan = mikhmon_voucher_template_saved($nama);
      if ($tersimpan !== '' && is_file($tersimpan)) {
        return $tersimpan;
      }
      return mikhmon_voucher_template_default($nama);
    }
  }

  /*
   * Isi template untuk ditampilkan di editor: hasil suntingan kalau ada,
   * kalau tidak bawaan. Mengembalikan string kosong kalau berkasnya tidak ada.
   */
  if (!function_exists('mikhmon_voucher_template_read')) {
    function mikhmon_voucher_template_read($nama)
    {
      $file = mikhmon_voucher_template_path($nama);
      if ($file === '' || !is_file($file)) {
        return '';
      }
      $isi = @file_get_contents($file);
      return $isi === false ? '' : $isi;
    }
  }

  /*
   * Simpan hasil suntingan.
   *
   * Ditolak kalau isinya kosong: tombol Simpan yang terkirim tanpa isi akan
   * menghapus template, dan itu lebih buruk daripada gagal menyimpan.
   */
  if (!function_exists('mikhmon_voucher_template_save')) {
    function mikhmon_voucher_template_save($nama, $isi)
    {
      $file = mikhmon_voucher_template_saved($nama);
      if ($file === '') {
        return false;
      }
      $isi = (string) $isi;
      if (trim($isi) === '') {
        return false;
      }

      $dir = mikhmon_voucher_template_dir();
      if (!is_dir($dir)) {
        return false;
      }

      /* Penjaga supaya berkas ini tidak berguna kalau dibuka langsung dari
       * browser (isinya memang bukan rahasia, tapi tetap tidak ada gunanya
       * dijalankan tanpa data voucher). */
      $penjaga = "<?php\n"
        . "if (isset(\$_SERVER['REQUEST_URI']) && substr(\$_SERVER['REQUEST_URI'], -" . strlen($nama . '.php') . ") == '" . $nama . ".php') { header('Location:./'); };\n"
        . "?>\n";

      $tmp = $file . '.' . getmypid() . '.' . mt_rand(100000, 999999) . '.tmp';
      if (@file_put_contents($tmp, $penjaga . $isi) === false) {
        return false;
      }
      if (@rename($tmp, $file)) {
        return true;
      }
      @unlink($tmp);
      return false;
    }
  }
}
