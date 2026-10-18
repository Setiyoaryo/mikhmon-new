<?php
/*
 * Pembentuk script on-login profil hotspot.
 *
 * Script ini menegakkan masa aktif voucher. Saat user login pertama kali, ia
 * membuat scheduler sesaat (nama "exp-<user>") dengan interval=validity dan
 * titik mulai = jam login, membaca next-run (jam login + validity), lalu
 * menuliskan hasilnya ke comment user dengan format mmm/dd/yyyy hh:mm:ss.
 * Monitor profil membaca comment itu untuk menghapus / menonaktifkan user yang
 * sudah lewat.
 *
 * CATATAN PENTING (v6.49)
 * -----------------------
 * Percobaan menghitung kadaluarsa langsung dengan aritmetika tanggal
 * (loop + array panjang) TIDAK dieksekusi oleh RouterOS v6.49 pada konteks
 * on-login profil, walau benar saat dijalankan sebagai /system script. Metode
 * scheduler ini terbukti berjalan pada router tersebut (145 voucher sudah
 * ter-stamp), jadi dipakai kembali.
 *
 * Dua perbaikan kritis pada metode ini:
 *   1. `start-time=$time` (titik mulai = jam login) WAJIB ada. Tanpa itu acuan
 *      waktunya bukan jam login dan masa aktif terpotong (voucher 1 hari yang
 *      dipakai malam bisa hanya berlaku beberapa jam).
 *   2. TIDAK ada `:error`. Dulu, bila next-run belum "siap" dalam 8 detik,
 *      script melempar error. Error pada on-login membuat login hotspot kadang
 *      gagal (login berulang). Sekarang bila belum siap, comment tidak ditulis
 *      dan dicoba lagi pada login berikutnya.
 */

if (!function_exists('mikhmon_onlogin_sched_head')) {
  function mikhmon_onlogin_iso_clock()
  {
    return ':if ([:len $date] = 10) do={ :set year [:pick $date 0 4]; :local ma ("jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec"); :local mi ([:tonum [:pick $date 5 7]] - 1); :set month ($ma->$mi); }; ';
  }

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
      . mikhmon_onlogin_iso_clock()
      /* Nama unik, supaya tidak bentrok dengan scheduler monitor profil. */
      . ':local schname ("exp-" . $user); '
      /* Bersihkan sisa scheduler bernama sama dari login sebelumnya yang gagal. */
      . '/sys sch remove [find where name=$schname]; '
      . '/sys sch add name=$schname disable=no start-date=$date start-time=$time interval="' . $validity . '"; ';
  }

  /*
   * Akhir blok scheduler: tunggu next-run siap (polling singkat), normalkan
   * formatnya ke MMM/DD/YYYY HH:MM:SS, lalu tulis ke comment user. Bila next-run
   * belum siap, jangan menulis dan jangan error. Scheduler sesaat selalu dihapus.
   *
   * @return string
   */
  function mikhmon_onlogin_sched_tail()
  {
    return ':local schid [ /sys sch find where name=$schname ]; '
      . ':local firstexp ("$date $time"); '
      . ':local exp [ /sys sch get $schid next-run]; '
      . ':local tries 0; '
      . ':while (($exp = "" or $exp = $time or $exp = $firstexp or $exp = ([:pick $date 0 6] . " " . $time)) and $tries < 8) do={ :delay 1s; :set exp [ /sys sch get $schid next-run]; :set tries ($tries + 1); }; '
      . ':local el [len $exp]; '
      /* Hanya tulis bila next-run sudah berupa waktu kadaluarsa (bukan jam login). */
      . ':if ($el > 0 and $exp != $time and $exp != $firstexp and $exp != ([:pick $date 0 6] . " " . $time)) do={ '
      . ':if ($el = 8) do={ '
      . ':local cdate $date; '
      . ':if ([:len $date] = 10) do={ :local ma ("jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec"); :local mi ([:tonum [:pick $date 5 7]] - 1); :local cm ($ma->$mi); :local cd [:pick $date 8 10]; :local cy [:pick $date 0 4]; :set cdate ("$cm/$cd/$cy"); }; '
      . '/ip hotspot user set comment="$cdate $exp" [find where name="$user"]; '
      . '} else={ '
      . ':local sp [:find $exp " "]; '
      . ':local dpart $exp; '
      . ':local tpart "00:00:00"; '
      . ':if ([:typeof $sp] != "nil") do={ :set dpart [:pic $exp 0 $sp]; :set tpart [:pick $exp ($sp + 1) [:len $exp]]; }; '
      . ':local mpart ""; '
      . ':local dp ""; '
      . ':local yp $year; '
      . ':if ([:pick $dpart 4 5] = "-") do={ '
      . ':local mn [:tonum [:pic $dpart 5 7]]; '
      . ':local ma ( "jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec" ); '
      . ':local mi ($mn - 1); '
      . ':set mpart ($ma->$mi); '
      . ':set dp [:pic $dpart 8 10]; '
      . ':set yp [:pic $dpart 0 4]; '
      . '} else={ '
      . ':set mpart [:pic $dpart 0 3]; '
      . ':set dp [:pic $dpart 4 6]; '
      . ':if ([:len $dpart] > 7) do={ :set yp [:pic $dpart 7 11]; } else={ :local ma ("jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec"); :if ([:find $ma $mpart] < [:find $ma $month]) do={ :set yp ([:tonum $year] + 1); }; }; '
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
    /* Nilai dari form disisipkan ke dalam script RouterOS. Bersihkan dulu agar
     * tanda kutip / titik koma / tanda dolar tidak bisa merusak script. Harga
     * boleh huruf+angka (mis. "free"), durasi memakai angka + unit + titik dua. */
    $expmode  = preg_replace('/[^a-zA-Z0-9]/', '', (string) $expmode);
    $price    = preg_replace('/[^a-zA-Z0-9]/', '', (string) $price);
    $sprice   = preg_replace('/[^a-zA-Z0-9]/', '', (string) $sprice);
    $getlock  = preg_replace('/[^a-zA-Z]/', '', (string) $getlock);
    $validity = preg_replace('/[^0-9a-zA-Z:]/', '', preg_replace('/\s+/', '', (string) $validity));

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
    }

    return array('onlogin' => $onlogin, 'mode' => $mode);
  }

  /*
   * Blok "tail" format scheduler generasi pertama (pra-hardening). Beku: dipakai
   * sebagai pola pencocokan saat menambal profil lama, jadi JANGAN diubah.
   *
   * @return string
   */
  function mikhmon_onlogin_legacy_tail()
  {
    return ':delay 5s; :local exp [ /sys sch get [ /sys sch find where name="$user" ] next-run]; '
      . ':local getxp [len $exp]; '
      . ':if ($getxp = 15) do={ :local d [:pic $exp 0 6]; :local t [:pic $exp 7 16]; :local s ("/"); :local exp ("$d$s$year $t"); /ip hotspot user set comment="$exp" [find where name="$user"];}; '
      . ':if ($getxp = 8) do={ /ip hotspot user set comment="$date $exp" [find where name="$user"];}; '
      . ':if ($getxp > 15) do={ /ip hotspot user set comment="$exp" [find where name="$user"];};'
      . ':delay 5s; '
      . '/sys sch remove [find where name="$user"]';
  }

  /*
   * Script monitor per-profil: dijalankan scheduler berkala, membaca comment
   * tiap user profil, dan menghapus / menonaktifkan yang sudah lewat masa
   * aktif. Satu-satunya sumber untuk tombol Tambah/Edit profil maupun pembuat
   * monitor otomatis di Fix Validity, supaya semuanya identik.
   *
   * @param string $pname nama profil (dipakai untuk filter & nama scheduler)
   * @param string $mode  "remove" atau "set limit-uptime=1s"
   *
   * @return string
   */
  function mikhmon_build_monitor($pname, $mode)
  {
    $pname = preg_replace('/[^A-Za-z0-9._-]/', '', (string) $pname);
    if ($mode !== 'set limit-uptime=1s') {
      $mode = 'remove';
    }
    return ':local dateint do={:local montharray ( "jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec" );:local days [ :pick $d 4 6 ];:local month [ :pick $d 0 3 ];:local year [ :pick $d 7 11 ];:local monthint ([ :find $montharray $month]);:local month ($monthint + 1);:if ( [len $month] = 1) do={:local zero ("0");:return [:tonum ("$year$zero$month$days")];} else={:return [:tonum ("$year$month$days")];}}; :local timeint do={ :local hours [ :pick $t 0 2 ]; :local minutes [ :pick $t 3 5 ]; :return ($hours * 60 + $minutes) ; }; :local date [ /system clock get date ]; :local time [ /system clock get time ]; :local today [$dateint d=$date] ; :local curtime [$timeint t=$time] ; :foreach i in [ /ip hotspot user find where profile="' . $pname . '" ] do={ :local comment [ /ip hotspot user get $i comment]; :local name [ /ip hotspot user get $i name]; :local gettime [:pic $comment 12 20]; :if ([:pic $comment 3] = "/" and [:pic $comment 6] = "/") do={:local expd [$dateint d=$comment] ; :local expt [$timeint t=$gettime] ; :if (($expd < $today and $expt < $curtime) or ($expd < $today and $expt > $curtime) or ($expd = $today and $expt < $curtime)) do={ [ /ip hotspot user ' . $mode . ' $i ]; [ /ip hotspot active remove [find where user=$name] ];}}}';
  }

  // Profile monitors parse comments as MMM/DD/YYYY. Newer routers expose the
  // system clock as YYYY-MM-DD; preserve the legacy parser for old comments.
  function mikhmon_monitor_upgrade($script)
  {
    $prefix = ':local dateint do={:local montharray';
    if (strpos($script, $prefix) !== 0 || strpos($script, '/ip hotspot user find where profile=') === false) {
      return $script;
    }
    $iso = ':if ([:pick $d 4 5] = "-") do={:return [:tonum ([:pick $d 0 4] . [:pick $d 5 7] . [:pick $d 8 10])];};';
    return str_replace($prefix, ':local dateint do={' . $iso . ':local montharray', $script);
  }

  /*
   * Naikkan satu script on-login lama ke format scheduler terkini.
   *
   * Semua format Mikhmon diawali prefix :put (",expmode,price,validity,
   * sprice,,lock,...). Parameter diambil dari prefix itu, lalu seluruh blok
   * on-login dibangun ulang dengan pembentuk yang sama seperti profil baru.
   * Field tambahan di prefix (mis. kode batch) dipertahankan apa adanya.
   * Format yang sudah memakai scheduler terkini dilewati (idempotent).
   *
   * @param string $onlogin script on-login saat ini
   * @param string $pname   nama profil (dipakai di record)
   *
   * @return array{onlogin:string, changed:bool}
   */
  function mikhmon_onlogin_upgrade($onlogin, $pname)
  {
    $onlogin = (string) $onlogin;
    if ($onlogin === '') {
      return array('onlogin' => $onlogin, 'changed' => false);
    }

    // Sudah memakai scheduler terkini? Tidak perlu apa-apa.
    if (strpos($onlogin, 'start-time=$time interval=') !== false && strpos($onlogin, '$schname') !== false) {
      return array('onlogin' => $onlogin, 'changed' => false);
    }

    // Bangun ulang dari prefix :put. Prefix dipertahankan apa adanya supaya
    // field tambahan (kode batch, versi, dsb.) tidak hilang.
    if (preg_match('/^\s*:put \("(.*?)"\)/', $onlogin, $m)) {
      $f = explode(',', $m[1]);
      $expmode  = isset($f[1]) ? trim($f[1]) : '';
      $price    = isset($f[2]) ? trim($f[2]) : '';
      $validity = isset($f[3]) ? trim($f[3]) : '';
      $sprice   = isset($f[4]) ? trim($f[4]) : '';
      $getlock  = isset($f[6]) ? trim($f[6]) : '';

      $validity = trim(preg_replace('/\s+/', '', $validity));

      if (in_array($expmode, array('rem', 'ntf', 'remc', 'ntfc'), true) && $validity !== '') {
        $lock = ($getlock == 'Enable')
          ? '; [:local mac $"mac-address"; /ip hotspot user set mac-address=$mac [find where name=$user]]'
          : '';
        $record = '; :local mac $"mac-address"; :set time [ /system clock get time ]; /system script add name="$date-|-$time-|-$user-|-'
          . $price . '-|-$address-|-$mac-|-' . $validity . '-|-' . $pname . '-|-$comment" owner="$month$year" source="$date" comment="mikhmon"';

        $built  = mikhmon_build_onlogin($expmode, $price, $validity, $sprice, $lock, $getlock, $record);
        $stdPut = ':put (",' . $expmode . ',' . $price . ',' . $validity . ',' . $sprice . ',,' . $getlock . ',")';

        if ($built['onlogin'] !== '' && strpos($built['onlogin'], $stdPut) === 0) {
          return array(
            'onlogin' => ':put ("' . $m[1] . '")' . substr($built['onlogin'], strlen($stdPut)),
            'changed' => true,
          );
        }
      }
    }

    return array('onlogin' => $onlogin, 'changed' => false);
  }
}
