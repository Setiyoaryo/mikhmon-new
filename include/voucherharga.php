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
 * Harga voucher per batch.
 *
 * Di Mikhmon asli, harga yang tercetak di voucher diambil dari PROFIL user itu.
 * Akibatnya semua voucher dari satu profil selalu berharga sama, dan kalau
 * harga profil diubah, voucher lama pun ikut berubah kalau dicetak ulang -
 * padahal penjualannya sudah terjadi dengan harga yang lama.
 *
 * Di sini harga yang diisi di halaman Generate dicatat per batch. Kuncinya
 * komentar batch - nilai yang sama yang dipakai halaman cetak untuk mengenali
 * batch itu. Halaman cetak memakai harga ini kalau ada, dan kembali ke harga
 * profil kalau tidak ada, jadi batch lama tidak berubah perilakunya.
 *
 * Berkasnya di data/ (di luar git): tidak ikut git pull dan tidak ter-commit.
 */

if (!defined('MIKHMON_VOUCHER_HARGA_LOADED')) {
  define('MIKHMON_VOUCHER_HARGA_LOADED', 1);

  /* Berapa batch terakhir yang disimpan, supaya berkasnya tidak tumbuh tanpa
   * henti. Batch yang lebih lama dari itu kembali memakai harga profil. */
  if (!defined('MIKHMON_VOUCHER_HARGA_SIMPAN')) {
    define('MIKHMON_VOUCHER_HARGA_SIMPAN', 400);
  }

  if (!function_exists('mikhmon_voucher_harga_file')) {
    function mikhmon_voucher_harga_file()
    {
      $dir = dirname(__FILE__) . '/../data/voucher';
      if (!is_dir($dir)) {
        @mkdir($dir, 0777, true);
      }
      return $dir . '/harga-batch.php';
    }
  }

  /* Seluruh catatan, urut dari yang paling lama. */
  if (!function_exists('mikhmon_voucher_harga_all')) {
    function mikhmon_voucher_harga_all()
    {
      $file = mikhmon_voucher_harga_file();
      if (!is_file($file)) {
        return array();
      }
      $isi = @include $file;
      return is_array($isi) ? $isi : array();
    }
  }

  /* Harga satu batch, atau null kalau batch itu tidak tercatat. */
  if (!function_exists('mikhmon_voucher_harga_get')) {
    function mikhmon_voucher_harga_get($kunci)
    {
      $kunci = (string) $kunci;
      if ($kunci === '') {
        return null;
      }
      $semua = mikhmon_voucher_harga_all();
      if (!isset($semua[$kunci]) || !is_array($semua[$kunci])) {
        return null;
      }
      return $semua[$kunci];
    }
  }

  /*
   * Catat harga satu batch. $harga berisi 'price' dan 'sprice' (angka saja).
   *
   * Keduanya disimpan apa adanya - yang kosong tetap kosong, bukan hanya yang
   * terisi. Halaman cetak perlu tahu kalau "selling price" batch ini memang
   * kosong: tanpa itu ia akan memakai selling price dari profil padahal batch
   * ini dijual dengan harga lain.
   */
  if (!function_exists('mikhmon_voucher_harga_put')) {
    function mikhmon_voucher_harga_put($kunci, $harga)
    {
      $kunci = trim((string) $kunci);
      if ($kunci === '' || !is_array($harga)) {
        return false;
      }

      $bersih = array();
      foreach (array('price', 'sprice') as $bidang) {
        $nilai = isset($harga[$bidang]) ? trim((string) $harga[$bidang]) : '';
        /* Hanya angka: nilainya dipakai di dalam template voucher, dan
         * sekaligus untuk memilih warna di template pelanggan. */
        $bersih[$bidang] = preg_replace('/[^0-9]/', '', $nilai);
      }

      /* Tidak ada yang diisi? Perlakukan seperti dulu: harga dari profil. */
      if ($bersih['price'] === '' && $bersih['sprice'] === '') {
        return false;
      }
      $bersih['waktu'] = time();

      $semua = mikhmon_voucher_harga_all();
      /* Kunci yang sama disimpan ulang tidak menumpuk; entri lamanya dibuang
       * dulu supaya urutannya tetap dari yang paling lama. */
      unset($semua[$kunci]);
      $semua[$kunci] = $bersih;

      if (count($semua) > MIKHMON_VOUCHER_HARGA_SIMPAN) {
        $semua = array_slice($semua, -MIKHMON_VOUCHER_HARGA_SIMPAN, null, true);
      }

      $isi = "<?php\n"
        . "/* Harga voucher per batch. Dibuat otomatis oleh halaman Generate; jangan di-commit. */\n"
        . "if (isset(\$_SERVER['REQUEST_URI']) && substr(\$_SERVER['REQUEST_URI'], -17) == 'harga-batch.php') { header('Location:./'); };\n"
        . "return " . var_export($semua, true) . ";\n";

      $file = mikhmon_voucher_harga_file();
      $tmp = $file . '.' . getmypid() . '.' . mt_rand(100000, 999999) . '.tmp';
      if (@file_put_contents($tmp, $isi) === false) {
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
