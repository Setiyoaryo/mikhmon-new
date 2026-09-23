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

      if (isset($harga['profile'])) {
        $bersih['profile'] = trim((string)$harga['profile']);
      }
      if (isset($harga['qty'])) {
        $bersih['qty'] = (int)$harga['qty'];
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

  /*
   * Menghapus entri batch dari registry lokal (harga-batch.php) saat seluruh
   * voucher dalam batch dihapus.
   */
  if (!function_exists('mikhmon_voucher_harga_del')) {
    function mikhmon_voucher_harga_del($kunci)
    {
      $kunci = trim((string) $kunci);
      if ($kunci === '') {
        return false;
      }
      $semua = mikhmon_voucher_harga_all();
      if (!is_array($semua) || !isset($semua[$kunci])) {
        return true;
      }
      unset($semua[$kunci]);

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

  /*
   * Mengambil ringkasan batch voucher untuk manajemen batch di hotspot.
   * Menggabungkan catatan lokal (harga-batch.php) dengan data user di router.
   */
  if (!function_exists('mikhmon_get_batches_summary')) {
    function mikhmon_get_batches_summary($API, $session)
    {
      include_once(dirname(__FILE__) . '/hscache.php');

      $registry = mikhmon_voucher_harga_all();
      $semuauser = mikhmon_hscache_hotspot_users($API, $session);

      $batches = array();

      // 1. Catatan yang ada di registry
      if (is_array($registry)) {
        foreach ($registry as $batchCode => $meta) {
          if (!is_array($meta)) continue;
          $batches[$batchCode] = array(
            'code'    => $batchCode,
            'profile' => isset($meta['profile']) ? (string)$meta['profile'] : '',
            'total'   => isset($meta['qty']) ? (int)$meta['qty'] : 0,
            'ready'   => 0,
            'used'    => 0,
            'price'   => isset($meta['price']) ? (string)$meta['price'] : '',
            'sprice'  => isset($meta['sprice']) ? (string)$meta['sprice'] : '',
            'time'    => isset($meta['waktu']) ? (int)$meta['waktu'] : 0,
          );
        }
      }

      // 2. Kumpulkan dari user di router (in-memory cache)
      if (is_array($semuauser)) {
        foreach ($semuauser as $u) {
          $comment = isset($u['comment']) ? trim((string)$u['comment']) : '';
          if ($comment === '') continue;

          // Pola komentar batch voucher: vc-... atau up-...
          $isBatch = (substr($comment, 0, 3) === 'vc-' || substr($comment, 0, 3) === 'up-');
          if (!$isBatch) continue;

          if (!isset($batches[$comment])) {
            $batches[$comment] = array(
              'code'    => $comment,
              'profile' => isset($u['profile']) ? (string)$u['profile'] : '',
              'total'   => 0,
              'ready'   => 0,
              'used'    => 0,
              'price'   => '',
              'sprice'  => '',
              'time'    => 0,
            );
          }

          $limituptime = isset($u['limit-uptime']) ? (string)$u['limit-uptime'] : '';
          $uptime = isset($u['uptime']) ? (string)$u['uptime'] : '';
          $bytesin = isset($u['bytes-in']) ? (int)$u['bytes-in'] : 0;
          $bytesout = isset($u['bytes-out']) ? (int)$u['bytes-out'] : 0;

          $isExpired = ($limituptime === '1s');
          $hasUsage = ($uptime !== '' && $uptime !== '0s' && $uptime !== '00:00:00') || ($bytesin > 0 || $bytesout > 0);

          if ($isExpired || $hasUsage) {
            $batches[$comment]['used']++;
          } else {
            $batches[$comment]['ready']++;
          }

          if (empty($batches[$comment]['profile']) && !empty($u['profile'])) {
            $batches[$comment]['profile'] = (string)$u['profile'];
          }
        }
      }

      // 3. Hitung total dan in-use
      foreach ($batches as $k => &$b) {
        $countedTotal = $b['ready'] + $b['used'];
        if ($b['total'] > 0) {
          $b['used'] = max($b['used'], $b['total'] - $b['ready']);
          $b['total'] = max($b['total'], $countedTotal);
        } else {
          $b['total'] = $countedTotal;
        }

        // Jika waktu belum tercatat, ambil dari tanggal di nama batch (contoh: vc-735-09.22.26-kopian atau vc-735-09.22.2026)
        if ($b['time'] === 0) {
          $parts = explode('-', $b['code']);
          foreach ($parts as $p) {
            if (preg_match('/^(\d{2})\.(\d{2})\.(\d{2}|\d{4})$/', $p, $m)) {
              $m_num = (int)$m[1];
              $d_num = (int)$m[2];
              $y_num = (strlen($m[3]) === 2) ? (2000 + (int)$m[3]) : (int)$m[3];
              $b['time'] = mktime(12, 0, 0, $m_num, $d_num, $y_num);
              break;
            }
          }
        }
      }
      unset($b);

      // Saring batch yang tidak memiliki voucher sama sekali (total == 0)
      $batches = array_filter($batches, function ($b) {
        return (isset($b['total']) && $b['total'] > 0);
      });

      // Urutkan dari batch terbaru
      uasort($batches, function ($a, $b) {
        if ($a['time'] === $b['time']) {
          return strcmp($b['code'], $a['code']);
        }
        return ($a['time'] > $b['time']) ? -1 : 1;
      });

      return $batches;
    }
  }
}
