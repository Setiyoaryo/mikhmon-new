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
 * 1. Scheduler WAJIB diberi start-time=$time. Tanpa itu acuan waktunya bukan
 *    jam login melainkan acuan default scheduler, sehingga masa aktif terpotong:
 *    voucher "1 hari" yang dipakai malam bisa hanya berlaku beberapa jam. Ini
 *    penyebab utama voucher pelanggan "cepat habis".
 *
 * 2. Nama scheduler per-user diberi awalan "exp-" (mis. exp-abc123), TIDAK lagi
 *    sama dengan nama user. Sebelumnya scheduler memakai nama user apa adanya,
 *    sehingga bisa bentrok dengan scheduler monitor profil (yang memakai nama
 *    profil). Bila nama user kebetulan sama dengan nama profil, perintah
 *    /sys sch remove di akhir bisa ikut menghapus scheduler monitor profil dan
 *    mematikan penegakan masa aktif untuk seluruh profil itu.
 *
 * 3. Masa tunggu dihapus. Sebelumnya on-login menahan 2 x 5 detik (10 detik)
 *    sebelum pelanggan bisa memakai internet. Sekarang next-run dibaca langsung,
 *    dan hanya menunggu bila memang belum siap (polling per 1 detik, maks 8).
 *
 * 4. Format next-run dinormalkan. RouterOS v7 mengembalikan next-run dalam
 *    format ISO "2024-06-28 01:30:11", sedangkan monitor profil mengharapkan
 *    "MMM/DD/YYYY HH:MM:SS" (mis. jun/28/2024 01:30:11). Script lama menyimpan
 *    ISO mentah sehingga monitor salah menghitung masa aktif. Sekarang semua
 *    format (ISO, MMM/DD/YYYY, MMM/DD, dan jam saja) diubah ke format monitor.
 */

if (!function_exists('mikhmon_onlogin_sched_head')) {
  /*
   * Awal blok scheduler: baca comment user, pastikan hanya untuk voucher/member,
   * siapkan tanggal & jam login, lalu buat scheduler sesaat dengan nama unik.
   *
   * @param string $validity masa aktif (sudah disanitasi, mis. "1d")
   * @return string
   */
  function mikhmon_onlogin_sched_head($validity)
  {
    return '{:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; '
      . ':local ucode [:pic $comment 0 2]; '
      . ':if ($ucode = "vc" or $ucode = "up" or $comment = "") do={ '
      . ':local date [ /system clock get date ]; '
      . ':local time [ /system clock get time ]; '
      . ':local year [ :pick $date 7 11 ]; '
      . ':local month [ :pick $date 0 3 ]; '
      /* Nama unik, supaya tidak bentrok dengan scheduler monitor profil. */
      . ':local schname ("exp-" . $user); '
      /* Bersihkan sisa scheduler bernama sama dari login sebelumnya yang gagal. */
      . '/sys sch remove [find where name=$schname]; '
      . '/sys sch add name=$schname disable=no start-date=$date start-time=$time interval="' . $validity . '"; ';
  }

  /*
   * Akhir blok scheduler: tunggu next-run siap (polling singkat), normalkan
   * formatnya ke MMM/DD/YYYY HH:MM:SS, tulis ke comment user, lalu hapus
   * scheduler sesaat.
   *
   * Dipakai bersama oleh pembuat script (fungsi ini) dan oleh tool migrasi
   * profil lama, supaya keduanya selalu menghasilkan blok yang identik.
   *
   * @return string
   */
  function mikhmon_onlogin_sched_tail()
  {
    return ':local exp [ /sys sch get [ /sys sch find where name=$schname ] next-run]; '
      . ':local tries 0; '
      . ':while ($exp = "" and $tries < 8) do={ :delay 1s; :set exp [ /sys sch get [ /sys sch find where name=$schname ] next-run]; :set tries ($tries + 1); }; '
      . ':local el [len $exp]; '
      . ':if ($el > 0) do={ '
      /* Jam saja (mis. "01:30:11") berarti kadaluarsa hari ini. */
      . ':if ([:find $exp "/"] < 0 and [:find $exp "-"] < 0) do={ '
      . '/ip hotspot user set comment="$date $exp" [find where name="$user"]; '
      . '} else={ '
      . ':local sp [:find $exp " "]; '
      . ':local dpart $exp; '
      . ':local tpart "00:00:00"; '
      . ':if ($sp >= 0) do={ :set dpart [:pic $exp 0 $sp]; :set tpart [:pick $exp ($sp + 1) [:len $exp]]; }; '
      . ':local mpart ""; '
      . ':local dp ""; '
      . ':local yp $year; '
      . ':if ([:find $dpart "-"] >= 0) do={ '
      /* ISO: YYYY-MM-DD -> ubah nomor bulan jadi nama bulan. */
      . ':local mn [:tonum [:pic $dpart 5 7]]; '
      . ':local ma ( "jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec" ); '
      . ':local mi ($mn - 1); '
      . ':set mpart ($ma->$mi); '
      . ':set dp [:pic $dpart 8 10]; '
      . ':set yp [:pic $dpart 0 4]; '
      . '} else={ '
      /* MMM/DD atau MMM/DD/YYYY. */
      . ':set mpart [:pic $dpart 0 3]; '
      . ':set dp [:pic $dpart 4 6]; '
      . ':if ([:len $dpart] > 7) do={ :set yp [:pic $dpart 7 11]; }; '
      . '}; '
      . '/ip hotspot user set comment="$mpart/$dp/$yp $tpart" [find where name="$user"]; '
      . '}; '
      . '}; '
      . '/sys sch remove [find where name=$schname]';
  }

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
      $base = $put . '; ' . mikhmon_onlogin_sched_head($validity) . mikhmon_onlogin_sched_tail();
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
