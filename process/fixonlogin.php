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
        } elseif (strpos($ol, ':local mdays (31,28,') !== false) {
          $cls = 'BARU';
        } elseif (strpos($ol, '$schname') !== false || strpos($ol, '/sys sch add') !== false || strpos($ol, 'start-date=$date') !== false || strpos($ol, ':local firstexp') !== false) {
          $cls = 'LAMA';
        } else {
          $cls = 'LAIN';
        }
        echo "===== [$cls] " . htmlspecialchars($pn) . "  (panjang=" . strlen($ol) . ") =====\n";
        echo htmlspecialchars($ol) . "\n\n";
      }
    }

    // Deteksi monitor kadaluarsa asing/ganda. Sistem Mikhmon lain (mis.
    // "Mikhmon Expire Monitor") memasang scheduler sendiri yang juga menghapus
    // user berdasarkan comment. Kalau berjalan bersamaan dengan monitor profil
    // dari panel ini, dua mesin bisa saling mendahului dan memotong masa aktif.
    // Tanda yang dicari khusus aksi kadaluarsa, supaya monitor lain (mis. Live
    // Report Monitor) tidak ikut dilaporkan.
    echo "\n\n===== SCHEDULER PEMANTAU KADALUARSA (periksa bentrok) =====\n";
    $schedulers = $API->comm('/system/scheduler/print');
    $profileNames = array();
    foreach ((array) $profiles as $p) {
      if (isset($p['name'])) { $profileNames[$p['name']] = true; }
    }
    $foreign = 0;
    foreach ((array) $schedulers as $sc) {
      $sn  = isset($sc['name']) ? $sc['name'] : '(tanpa nama)';
      $com = isset($sc['comment']) ? $sc['comment'] : '';
      $ev  = isset($sc['on-event']) ? $sc['on-event'] : '';
      $isMonitor = (strpos($ev, 'hotspot active remove') !== false)
        || (strpos($ev, 'limit-uptime=1s') !== false)
        || (strpos($ev, 'expireMonitor') !== false);
      if (!$isMonitor) { continue; }
      // Monitor profil panel ini memakai nama = nama profil. Sisanya = asing.
      $kind = isset($profileNames[$sn]) ? 'PANEL' : 'ASING/LAMA';
      if ($kind !== 'PANEL') { $foreign++; }
      echo "[$kind] name=" . htmlspecialchars($sn)
        . "  interval=" . htmlspecialchars(isset($sc['interval']) ? $sc['interval'] : '')
        . "  comment=" . htmlspecialchars($com) . "\n";
    }
    echo "\nTotal scheduler pemantau: " . (is_array($schedulers) ? count($schedulers) : 0)
      . "  |  ASING/LAMA: $foreign\n";
    if ($foreign > 0) {
      echo "PERINGATAN: ada $foreign scheduler pemantau kadaluarsa dari sistem lain.\n";
      echo "Jalankan HANYA satu mesin kadaluarsa. Hapus yang asing agar tidak bentrok.\n";
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
  $failed = 0;
  if (!empty($updates)) {
    $result = mikhmon_bulk_profile_onlogin_set($API, $updates);
    if (!empty($result['backend'])) {
      $fixed = (int) $result['updated'];
      $failed = (int) $result['failed'];
    } else {
      // Backend Go tidak terjangkau: pakai jalur lama, satu per satu, supaya
      // tombol ini tetap bekerja walau service sedang mati.
      foreach ($updates as $u) {
        $API->error_str = '';
        $reply = $API->comm("/ip/hotspot/user/profile/set", array(
          ".id"      => $u['id'],
          "on-login" => $u['onlogin'],
        ));
        if (isset($reply['!trap']) || isset($reply['!fatal']) || $API->error_str !== '') {
          $failed++;
        } else {
          $fixed++;
        }
      }
    }
  }

  // Upgrade only recognized Mikhmon profile monitors. Run their writes through
  // Go as a batch as well, including monitors whose on-login was already fixed.
  $profileNames = array();
  foreach ((array) $getprofile as $prof) {
    if (isset($prof['name'])) { $profileNames[$prof['name']] = true; }
  }
  $schedulers = $API->comm('/system/scheduler/print', array('.proplist' => '.id,name,on-event'));
  $monitorCommands = array();
  foreach ((array) $schedulers as $scheduler) {
    if (!isset($scheduler['.id'], $scheduler['name'], $scheduler['on-event']) || !isset($profileNames[$scheduler['name']])) { continue; }
    $event = mikhmon_monitor_upgrade($scheduler['on-event']);
    if ($event !== $scheduler['on-event']) {
      $monitorCommands[] = array('/system/scheduler/set', '=.id=' . $scheduler['.id'], '=on-event=' . $event);
    }
  }
  $monitorsFixed = 0;
  if ($monitorCommands) {
    $response = mikhmon_api_post('/v1/exec', array(
      'session' => $API->session,
      'sentences' => $monitorCommands,
      'timeout_ms' => max(1000, (int) $API->timeout * 1000),
    ), mikhmon_api_exec_timeout());
    $results = is_array($response) && !empty($response['ok']) && isset($response['results']) ? $response['results'] : array();
    foreach ($results as $res) {
      if (empty($res['error']) && !empty($res['sentences'])) { $monitorsFixed++; }
    }
    $failed += count($monitorCommands) - $monitorsFixed;
  }

  // Lengkapi monitor profil yang hilang. Dulu monitor hanya dibuat saat profil
  // disimpan, jadi profil lama bisa tidak punya monitor sama sekali dan
  // mengandalkan sistem kadaluarsa pihak ketiga. Sekarang panel memastikan
  // setiap profil ber-expiry punya monitor sendiri.
  $existingMonitors = array();
  foreach ((array) $schedulers as $scheduler) {
    if (isset($scheduler['name'])) { $existingMonitors[$scheduler['name']] = true; }
  }
  $monitorsCreated = 0;
  foreach ((array) $getprofile as $prof) {
    $pname   = isset($prof['name']) ? $prof['name'] : '';
    $onlogin = isset($prof['on-login']) ? $prof['on-login'] : '';
    if ($pname === '' || $onlogin === '' || isset($existingMonitors[$pname])) { continue; }
    if (!preg_match('/^\s*:put \("(.*?)"\)/', $onlogin, $m)) { continue; }
    $f = explode(',', $m[1]);
    $expmode = isset($f[1]) ? trim($f[1]) : '';
    if (!in_array($expmode, array('rem', 'ntf', 'remc', 'ntfc'), true)) { continue; }
    $pmode = ($expmode === 'ntf' || $expmode === 'ntfc') ? 'set limit-uptime=1s' : 'remove';
    $event = mikhmon_monitor_upgrade(mikhmon_build_monitor($pname, $pmode));
    $randstarttime = "0" . rand(1, 5) . ":" . rand(10, 59) . ":" . rand(10, 59);
    $randinterval  = "00:02:" . rand(10, 59);
    $API->error_str = '';
    $reply = $API->comm("/system/scheduler/add", array(
      "name"       => "$pname",
      "start-time" => "$randstarttime",
      "interval"   => "$randinterval",
      "on-event"   => "$event",
      "disabled"   => "no",
      "comment"    => "Monitor Profile $pname",
    ));
    if (isset($reply['!trap']) || isset($reply['!fatal']) || $API->error_str !== '') {
      $failed++;
    } else {
      $monitorsCreated++;
    }
  }

  echo "<script>window.location='./?hotspot=user-profiles&fixdone=" . (int) $fixed
    . "&fixskip=" . (int) $skipped . "&fixmonitor=" . ($monitorsFixed + $monitorsCreated) . "&fixfail=" . $failed . "&session=" . $session . "'</script>";
}
?>
