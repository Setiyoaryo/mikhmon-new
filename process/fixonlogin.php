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
 *  along with this program; if not, write to the Free Software
 *  Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
 */
session_start();
// hide all error
error_reporting(0);

/*
 * Perbaiki script on-login profil yang SUDAH ADA di router.
 *
 * Ada dua generasi perbaikan:
 *   - Tahap 1: menambahkan start-time=$time supaya masa aktif tidak terpotong.
 *   - Tahap 2: nama scheduler unik (exp-<user>), menghapus delay 10 detik, dan
 *     menormalkan format next-run agar monitor masa aktif menghitung dengan benar.
 *
 * Halaman ini menambal script lama dengan penggantian yang sempit, tanpa
 * mengubah bagian lain (harga, record, lock) sama sekali. Semua langkah aman
 * dijalankan berulang: profil yang sudah benar dilewati, dan profil yang
 * polanya tidak dikenal juga dilewati (tidak pernah ditebak-tebak).
 *
 * Semua perubahan dikirim sekaligus ke service Go
 * (mikhmon_bulk_profile_onlogin_set) agar diproses paralel - dulu satu
 * round-trip per profil, sehingga tombol ini lambat di router berprofil banyak.
 * Bila service Go tidak terjangkau, otomatis kembali ke jalur satu-per-satu.
 */

if (!isset($_SESSION["mikhmon"])) {
  header("Location:../admin.php?id=login");
} else {
  // Mode laporan (read-only): ?fix-onlogin=report
  // Menampilkan script on-login tiap profil apa adanya supaya bisa diperiksa
  // dari browser tanpa akses langsung ke router. Tidak mengubah apa pun.
  if (isset($_GET['fix-onlogin']) && $_GET['fix-onlogin'] === 'report') {
    $profiles = $API->comm("/ip/hotspot/user/profile/print");
    header('Content-Type: text/html; charset=utf-8');
    echo "<pre style='white-space:pre-wrap;word-break:break-all;background:#111;color:#ccc;padding:14px;font-size:12px'>";
    echo "LAPORAN ON-LOGIN PROFIL (read-only, tidak mengubah apa pun)\n";
    echo "Total: " . (is_array($profiles) ? count($profiles) : 0) . " profil\n";
    echo "Keterangan: BARU=sudah diperbaiki  LAMA=pakai scheduler belum diperbaiki  LAIN=format tak dikenal  KOSONG=tanpa script\n\n";
    if (is_array($profiles)) {
      foreach ($profiles as $p) {
        $pn = isset($p['name']) ? $p['name'] : '(tanpa nama)';
        $ol = isset($p['on-login']) ? $p['on-login'] : '';
        if ($ol === '') {
          $cls = 'KOSONG';
        } elseif (strpos($ol, '$schname') !== false) {
          $cls = 'BARU';
        } elseif (strpos($ol, '/sys sch add') !== false || strpos($ol, 'start-date=$date') !== false) {
          $cls = 'LAMA';
        } else {
          $cls = 'LAIN';
        }
        echo "===== [$cls] " . htmlspecialchars($pn) . "  (panjang=" . strlen($ol) . ") =====\n";
        echo htmlspecialchars($ol) . "\n\n";
      }
    }
    echo "</pre>";
    exit;
  }

  include_once(dirname(__FILE__) . '/../include/onlogin.php');

  // Semua aturan upgrade on-login ada di include/onlogin.php
  // (mikhmon_onlogin_upgrade) supaya hanya ada satu sumber kebenaran dan
  // mudah diuji. Di sini kita hanya membaca profil, memanggilnya, lalu
  // mengirim semua perubahan sekaligus.
  $getprofile = $API->comm("/ip/hotspot/user/profile/print");

  $updates = array();
  $skipped = 0;

  if (is_array($getprofile)) {
    foreach ($getprofile as $prof) {
      $pid     = isset($prof['.id']) ? $prof['.id'] : '';
      $onlogin = isset($prof['on-login']) ? $prof['on-login'] : '';
      $pname   = isset($prof['name']) ? $prof['name'] : '';

      if ($pid === '' || $onlogin === '') {
        $skipped++;
        continue;
      }

      $res = mikhmon_onlogin_upgrade($onlogin, $pname);
      if (!empty($res['changed'])) {
        // Kumpulkan dulu; semua update dikirim sekaligus supaya service Go
        // menjalankannya paralel (dulu satu round-trip per profil = lambat).
        $updates[] = array('id' => $pid, 'onlogin' => $res['onlogin']);
      } else {
        $skipped++;
      }
    }
  }

  $fixed = 0;
  if (!empty($updates)) {
    $result = mikhmon_bulk_profile_onlogin_set($API, $updates);
    if (!empty($result['backend'])) {
      $fixed = (int) $result['updated'];
    } else {
      // Backend Go tidak terjangkau: pakai jalur lama, satu per satu, supaya
      // tombol ini tetap bekerja walau service sedang mati.
      foreach ($updates as $u) {
        $API->comm("/ip/hotspot/user/profile/set", array(
          ".id"      => $u['id'],
          "on-login" => $u['onlogin'],
        ));
        $fixed++;
      }
    }
  }

  echo "<script>window.location='./?hotspot=user-profiles&fixdone=" . (int) $fixed
    . "&fixskip=" . (int) $skipped . "&session=" . $session . "'</script>";
}
?>
