<?php
/*
 * Pembentuk script on-login profil hotspot.
 *
 * Script ini yang menegakkan masa aktif voucher: saat user login pertama kali,
 * ia menghitung waktu kadaluarsa (jam login + validity) dan menuliskannya ke
 * comment user dengan format mmm/dd/yyyy hh:mm:ss diikuti penanda N (notice)
 * atau X (remove). Scheduler monitor profil membaca comment itu untuk
 * menghapus / menonaktifkan user yang sudah lewat.
 *
 * PERBAIKAN PENTING (pengganti metode lama)
 * -----------------------------------------
 * Versi lama menghitung kadaluarsa dengan cara membuat scheduler sesaat lalu
 * MEMBACA `next-run` dan mem-parse string-nya berdasarkan posisi karakter.
 * Cara itu rapuh:
 *   - format next-run bisa beda antar versi RouterOS (ISO / mmm/dd/yyyy) dan
 *     bisa time-only atau date-only tanpa jam;
 *   - kalau next-run belum "siap" dalam 8 detik, script melempar `:error`, dan
 *     error pada on-login membuat login hotspot kadang gagal (login berulang);
 *   - asumsi "time-only berarti hari ini" salah untuk login malam yang lewat
 *     tengah malam, sehingga masa aktif terpotong (3 jam jadi 1 jam).
 *
 * Sekarang kadaluarsa dihitung langsung dengan aritmetika tanggal/waktu di
 * dalam script: tidak ada scheduler sesaat, tidak ada parsing next-run, tidak
 * ada `:error`, dan tidak ada ketergantungan format. Hasilnya deterministik
 * untuk v6 (mmm/dd/yyyy) maupun v7 (yyyy-mm-dd).
 */

if (!function_exists('mikhmon_onlogin_body')) {
  /*
   * Blok RouterOS yang membaca jam login, menambahkan validity, lalu menulis
   * comment kadaluarsa. Blok ini dibuka dengan `{` dan `:if (...) do={` yang
   * penutupnya disediakan pemanggil (lihat mikhmon_build_onlogin), supaya
   * record & lock tetap bisa disisipkan di dalam blok yang sama.
   *
   * @param string $validity masa aktif RouterOS (mis. "4h", "1d", "30d")
   * @param bool   $notice   true untuk mode notice (penanda " N"),
   *                         false untuk mode remove (penanda " X")
   *
   * @return string
   */
  function mikhmon_onlogin_body($validity, $notice)
  {
    $flag = $notice ? ' N' : ' X';
    $val  = (string) $validity;

    $s  = '{:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; ';
    $s .= ':local ucode [:pic $comment 0 2]; ';
    /* Hanya voucher/member baru (comment masih kosong / berawalan vc / up). */
    $s .= ':if ($ucode = "vc" or $ucode = "up" or $comment = "") do={ ';
    $s .= ':local date [ /system clock get date ]; ';
    $s .= ':local time [ /system clock get time ]; ';
    $s .= ':local ma ("jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec"); ';
    $s .= ':local year 0; :local monum 0; :local day 0; ';
    /* Tanggal router: v7 = ISO "yyyy-mm-dd", v6 = "mmm/dd/yyyy". */
    $s .= ':if ([:pick $date 4 5] = "-") do={ :set year [:tonum [:pick $date 0 4]]; :set monum [:tonum [:pick $date 5 7]]; :set day [:tonum [:pic $date 8 10]]; } ';
    $s .= 'else={ :set monum ([:find $ma [:pic $date 0 3]] + 1); :set day [:tonum [:pic $date 4 6]]; :set year [:tonum [:pic $date 7 11]]; }; ';
    $s .= ':local hh [:tonum [:pic $time 0 2]]; :local mi [:tonum [:pic $time 3 5]]; :local ss [:tonum [:pic $time 6 8]]; ';
    /* Ubah validity (mis. "4h", "1d2h30m") menjadi total detik. */
    $s .= ':local val "' . $val . '"; ';
    $s .= ':local vs 0; :local num ""; :local i 0; ';
    $s .= ':while ($i < [:len $val]) do={ ';
    $s .= ':local ch [:pic $val $i ($i+1)]; ';
    $s .= ':if ([:typeof [:find "0123456789" $ch]] != "nil") do={ :set num ($num . $ch); } ';
    $s .= 'else={ :local n 1; :if ([:len $num] > 0) do={ :set n [:tonum $num]; }; ';
    $s .= ':if ($ch = "d") do={ :set vs ($vs + $n*86400); }; ';
    $s .= ':if ($ch = "h") do={ :set vs ($vs + $n*3600); }; ';
    $s .= ':if ($ch = "m") do={ :set vs ($vs + $n*60); }; ';
    $s .= ':if ($ch = "s") do={ :set vs ($vs + $n); }; ';
    $s .= ':if ($ch = "w") do={ :set vs ($vs + $n*604800); }; :set num ""; }; ';
    $s .= ':set i ($i+1); }; ';
    /* Tambahkan detik ke jam login, pisahkan kelebihan ke hari/jam/menit
     * (pakai pengurangan integer; hindari :floor yang tidak terdokumentasi). */
    $s .= ':local tot ($hh*3600 + $mi*60 + $ss + $vs); ';
    $s .= ':local ad 0; :while ($tot >= 86400) do={ :set tot ($tot - 86400); :set ad ($ad + 1); }; ';
    $s .= ':local nh 0; :while ($tot >= 3600) do={ :set tot ($tot - 3600); :set nh ($nh + 1); }; ';
    $s .= ':local nmn 0; :while ($tot >= 60) do={ :set tot ($tot - 60); :set nmn ($nmn + 1); }; ';
    $s .= ':local ns $tot; ';
    /* Geser tanggal sebanyak kelebihan hari, dengan panjang bulan + kabisat. */
    $s .= ':local mdays (31,28,31,30,31,30,31,31,30,31,30,31); ';
    $s .= ':while ($ad > 0) do={ ';
    $s .= ':local dim ($mdays->($monum-1)); ';
    $s .= ':if ($monum = 2 and ($year % 4) = 0 and (($year % 100) != 0 or ($year % 400) = 0)) do={ :set dim 29; }; ';
    $s .= ':set day ($day+1); ';
    $s .= ':if ($day > $dim) do={ :set day 1; :set monum ($monum+1); :if ($monum > 12) do={ :set monum 1; :set year ($year+1); }; }; ';
    $s .= ':set ad ($ad-1); }; ';
    /* Zero-pad dua digit tanpa fungsi khusus. */
    $s .= ':local pd [:tostr $day]; :if ($day < 10) do={ :set pd ("0" . $pd); }; ';
    $s .= ':local ph [:tostr $nh]; :if ($nh < 10) do={ :set ph ("0" . $ph); }; ';
    $s .= ':local pm [:tostr $nmn]; :if ($nmn < 10) do={ :set pm ("0" . $pm); }; ';
    $s .= ':local ps [:tostr $ns]; :if ($ns < 10) do={ :set ps ("0" . $ps); }; ';
    /* $month/$year dipakai lagi oleh blok record di belakang. */
    $s .= ':local month ($ma->($monum-1)); ';
    $s .= ':ip hotspot user set comment=("$month/$pd/$year $ph:$pm:$ps' . $flag . '") [find where name="$user"]';
    return $s;
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

    $onlogin = '';
    $mode = '';

    if ($validity !== '' && in_array($expmode, array('rem', 'ntf', 'remc', 'ntfc'), true)) {
      $notice = ($expmode === 'ntf' || $expmode === 'ntfc');
      $base  = $put . '; ' . mikhmon_onlogin_body($validity, $notice);
      $close = '}}';

      if ($expmode === 'rem') {
        $onlogin = $base . $lock . $close;
        $mode = 'remove';
      } elseif ($expmode === 'ntf') {
        $onlogin = $base . $lock . $close;
        $mode = 'set limit-uptime=1s';
      } elseif ($expmode === 'remc') {
        $onlogin = $base . $record . $lock . $close;
        $mode = 'remove';
      } elseif ($expmode === 'ntfc') {
        $onlogin = $base . $record . $lock . $close;
        $mode = 'set limit-uptime=1s';
      }
    } elseif ($expmode === '0' && $price != '') {
      $onlogin = ':put (",,' . $price . ',,,noexp,' . $getlock . ',")' . $lock;
    }

    return array('onlogin' => $onlogin, 'mode' => $mode);
  }

  /*
   * Blok "tail" format scheduler generasi lama (pra-hardening). Beku: dipakai
   * sebagai penanda pengenalan script lama, jangan diubah.
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
   * Naikkan satu script on-login lama ke format hitung-langsung terkini.
   *
   * Semua format lama Mikhmon diawali prefix :put (",expmode,price,validity,
   * sprice,,lock,...). Parameter diambil dari prefix itu, lalu seluruh blok
   * on-login dibangun ulang dengan pembentuk yang sama seperti profil baru.
   * Field tambahan di prefix (mis. kode batch) dipertahankan apa adanya.
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

    // Sudah memakai format hitung-langsung terkini? Tidak perlu apa-apa.
    if (strpos($onlogin, ':local mdays (31,28,') !== false
        && strpos($onlogin, '[/ip hotspot user find where name="$user"] comment') !== false) {
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
