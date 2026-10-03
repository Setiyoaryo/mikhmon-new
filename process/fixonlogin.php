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
 * Profil yang dibuat sebelum perbaikan start-time masih membawa script lama,
 * sehingga masa aktif voucher terpotong (voucher 1 hari yang dipakai malam
 * hanya berlaku beberapa jam). Menyimpan ulang profil lewat halaman Edit akan
 * memperbaikinya, tetapi kalau profilnya banyak itu melelahkan.
 *
 * Halaman ini menambal script lama dengan tiga penggantian yang sempit, tanpa
 * mengubah bagian lain sama sekali:
 *
 *   1. tambahkan :local time [ /system clock get time ]; sebelum dipakai
 *   2. tambahkan start-time=$time pada /sys sch add
 *   3. rapikan :local time [ ... ] pada script Record menjadi :set time [ ... ]
 *
 * Aman dijalankan berulang: profil yang sudah benar dilewati, dan profil yang
 * polanya tidak dikenal juga dilewati (tidak pernah ditebak-tebak).
 */

if (!isset($_SESSION["mikhmon"])) {
  header("Location:../admin.php?id=login");
} else {
  $getprofile = $API->comm("/ip/hotspot/user/profile/print");

  $fixed = 0;
  $skipped = 0;

  if (is_array($getprofile)) {
    foreach ($getprofile as $prof) {
      $pid = isset($prof['.id']) ? $prof['.id'] : '';
      $onlogin = isset($prof['on-login']) ? $prof['on-login'] : '';

      if ($pid === '' || $onlogin === '') {
        $skipped++;
        continue;
      }

      // Sudah punya start-time? Berarti sudah benar.
      if (strpos($onlogin, 'start-time=$time') !== false) {
        $skipped++;
        continue;
      }

      // Hanya tambal profil yang memang memakai scheduler on-login. Profil
      // mode "None" tidak punya scheduler, jadi tidak perlu diapa-apakan.
      if (strpos($onlogin, '/sys sch add') === false && strpos($onlogin, 'start-date=$date') === false) {
        $skipped++;
        continue;
      }

      $new = $onlogin;

      // 1. Deklarasikan waktu login sebelum dipakai.
      $new = str_replace(
        ':local date [ /system clock get date ];:local year',
        ':local date [ /system clock get date ];:local time [ /system clock get time ];:local year',
        $new
      );

      // 2. Pakai jam login sebagai acuan scheduler (inti perbaikan).
      $new = str_replace(
        'disable=no start-date=$date interval=',
        'disable=no start-date=$date start-time=$time interval=',
        $new
      );

      // 3. Rapikan script Record.
      $new = str_replace(
        ':local time [/system clock get time ]',
        ':set time [ /system clock get time ]',
        $new
      );

      if ($new === $onlogin) {
        // Pola tidak cocok: jangan menebak, biarkan apa adanya.
        $skipped++;
        continue;
      }

      $API->comm("/ip/hotspot/user/profile/set", array(
        ".id" => $pid,
        "on-login" => $new,
      ));
      $fixed++;
    }
  }

  echo "<script>window.location='./?hotspot=user-profiles&fixdone=" . (int) $fixed
    . "&fixskip=" . (int) $skipped . "&session=" . $session . "'</script>";
}
?>
