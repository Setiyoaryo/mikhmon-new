<?php
/*
 * Pembentuk script on-login profil hotspot.
 *
 * Script ini yang menegakkan masa aktif voucher: saat user login, ia membuat
 * scheduler sesaat untuk menghitung waktu kadaluarsa, lalu menyimpan hasilnya
 * di comment user. Scheduler monitor profil (dibuat terpisah di halaman profil)
 * memakai comment itu untuk menghapus / menonaktifkan user yang sudah lewat.
 *
 * Logika ini dulu disalin di dua berkas (hotspot/adduserprofile.php dan
 * hotspot/userprofilebyname.php), sehingga rawan berbeda. Sekarang keduanya
 * memanggil fungsi di sini supaya selalu identik.
 *
 * PERBAIKAN PENTING
 * -----------------
 * Scheduler WAJIB diberi start-time=$time. Tanpa itu acuan waktunya bukan jam
 * login melainkan acuan default scheduler, sehingga masa aktif terpotong:
 * voucher "1 hari" yang dipakai malam bisa hanya berlaku beberapa jam. Ini
 * penyebab utama voucher pelanggan "cepat habis".
 */

if (!function_exists('mikhmon_build_onlogin')) {
  /*
   * @param string $expmode  rem | ntf | remc | ntfc | 0
   * @param string $price    harga dari profil (angka)
   * @param string $validity masa aktif, format durasi RouterOS (mis. 1d, 12h, 30m)
   * @param string $sprice   harga jual dari profil (angka)
   * @param string $lock     potongan script lock MAC ("" kalau nonaktif)
   * @param string $getlock  nilai opsi lock ("Enable" / "Disable")
   * @param string $record   potongan script pencatatan (mode ...c)
   *
   * @return array{onlogin:string, mode:string}
   */
  function mikhmon_build_onlogin($expmode, $price, $validity, $sprice, $lock, $getlock, $record)
  {
    /* Spasi di dalam durasi membuat /sys sch add gagal, jadi dibuang dulu. */
    $validity = trim(preg_replace('/\s+/', '', (string) $validity));

    $put = ':put (",' . $expmode . ',' . $price . ',' . $validity . ',' . $sprice . ',,' . $getlock . ',")';

    /* Blok scheduler hanya dibuat kalau masa aktif terisi. Interval kosong
     * membuat /sys sch add gagal, dan masa aktif tidak akan pernah tercatat. */
    if ($validity !== '') {
      $sched = '{:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; '
        . ':local ucode [:pic $comment 0 2]; '
        . ':if ($ucode = "vc" or $ucode = "up" or $comment = "") do={ '
        . ':local date [ /system clock get date ];'
        . ':local time [ /system clock get time ];'
        . ':local year [ :pick $date 7 11 ];'
        . ':local month [ :pick $date 0 3 ]; '
        . '/sys sch add name="$user" disable=no start-date=$date start-time=$time interval="' . $validity . '"; '
        . ':delay 5s; '
        . ':local exp [ /sys sch get [ /sys sch find where name="$user" ] next-run]; '
        . ':local getxp [len $exp]; '
        . ':if ($getxp = 15) do={ :local d [:pic $exp 0 6]; :local t [:pic $exp 7 16]; :local s ("/"); :local exp ("$d$s$year $t"); /ip hotspot user set comment="$exp" [find where name="$user"];}; '
        . ':if ($getxp = 8) do={ /ip hotspot user set comment="$date $exp" [find where name="$user"];}; '
        . ':if ($getxp > 15) do={ /ip hotspot user set comment="$exp" [find where name="$user"];};'
        . ':delay 5s; '
        . '/sys sch remove [find where name="$user"]';
      $base = $put . '; ' . $sched;
      $close = '}}';
    } else {
      $base = $put;
      $close = '';
    }

    $onlogin = '';
    $mode = '';

    if ($expmode == "rem") {
      $onlogin = $base . $lock . $close;
      $mode = "remove";
    } elseif ($expmode == "ntf") {
      $onlogin = $base . $lock . $close;
      $mode = "set limit-uptime=1s";
    } elseif ($expmode == "remc") {
      $onlogin = $base . $record . $lock . $close;
      $mode = "remove";
    } elseif ($expmode == "ntfc") {
      $onlogin = $base . $record . $lock . $close;
      $mode = "set limit-uptime=1s";
    } elseif ($expmode == "0" && $price != "") {
      $onlogin = ':put (",,' . $price . ',,,noexp,' . $getlock . ',")' . $lock;
    } else {
      $onlogin = "";
    }

    return array('onlogin' => $onlogin, 'mode' => $mode);
  }
}
