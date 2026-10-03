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
  include_once(dirname(__FILE__) . '/../include/onlogin.php');

  // Blok "tail" lama (pra-hardening), yaitu bagian setelah /sys sch add.
  // Dipakai sebagai pola pencocokan saat migrasi; teks ini beku (tidak berubah).
  $legacyTail = ':delay 5s; :local exp [ /sys sch get [ /sys sch find where name="$user" ] next-run]; '
    . ':local getxp [len $exp]; '
    . ':if ($getxp = 15) do={ :local d [:pic $exp 0 6]; :local t [:pic $exp 7 16]; :local s ("/"); :local exp ("$d$s$year $t"); /ip hotspot user set comment="$exp" [find where name="$user"];}; '
    . ':if ($getxp = 8) do={ /ip hotspot user set comment="$date $exp" [find where name="$user"];}; '
    . ':if ($getxp > 15) do={ /ip hotspot user set comment="$exp" [find where name="$user"];};'
    . ':delay 5s; '
    . '/sys sch remove [find where name="$user"]';

  $newTail = mikhmon_onlogin_sched_tail();

  $getprofile = $API->comm("/ip/hotspot/user/profile/print");

  $updates = array();
  $skipped = 0;

  if (is_array($getprofile)) {
    foreach ($getprofile as $prof) {
      $pid = isset($prof['.id']) ? $prof['.id'] : '';
      $onlogin = isset($prof['on-login']) ? $prof['on-login'] : '';

      if ($pid === '' || $onlogin === '') {
        $skipped++;
        continue;
      }

      // Sudah sepenuhnya baru (start-time + nama scheduler unik)? Lewati.
      if (strpos($onlogin, 'start-time=$time') !== false && strpos($onlogin, '$schname') !== false) {
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

      // 1. Deklarasikan waktu login sebelum dipakai (kalau belum ada).
      $new = str_replace(
        ':local date [ /system clock get date ];:local year',
        ':local date [ /system clock get date ];:local time [ /system clock get time ];:local year',
        $new
      );

      // 2. Pakai jam login sebagai acuan scheduler (inti perbaikan Tahap 1).
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

      // 4 + 5. Tahap 2: upgrade tail (delay + parsing) dan nama scheduler,
      //         dilakukan bersamaan supaya tidak pernah setengah jalan.
      $beforeTail = $new;
      $new = str_replace($legacyTail, $newTail, $new);
      if ($new !== $beforeTail) {
        $new = str_replace(
          '/sys sch add name="$user" disable=no',
          ':local schname ("exp-" . $user); /sys sch remove [find where name=$schname]; /sys sch add name=$schname disable=no',
          $new
        );
      }

      if ($new === $onlogin) {
        // Pola tidak cocok: jangan menebak, biarkan apa adanya.
        $skipped++;
        continue;
      }

      // Kumpulkan dulu; semua update dikirim sekaligus supaya service Go
      // menjalankannya paralel (dulu satu round-trip per profil = lambat).
      $updates[] = array('id' => $pid, 'onlogin' => $new);
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
