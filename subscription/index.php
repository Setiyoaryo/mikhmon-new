<?php
/*
 *  Mikhmon Nocify - halaman langganan / sewa bulanan.
 *
 *  Copyright (C) 2018 Laksamadi Guko.        (Mikhmon asli)
 *  Copyright (C) 2024 NOCIFY.                (tambahan fork ini)
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation; either version 2 of the License, or
 *  (at your option) any later version.
 *
 *  Halaman ini hanya menampilkan status dari portal. Semua pembayaran dan
 *  perpanjangan dilakukan di portal, bukan di panel ini.
 */

if (session_status() === PHP_SESSION_NONE) {
  session_start();
}

if (!isset($_SESSION["mikhmon"])) {
  echo "<script>window.location='./admin.php?id=login'</script>";
  return;
}

if (!defined('MIKHMON_SUBSCRIPTION_LOADED')) {
  include_once(dirname(__FILE__) . '/../include/subscription.php');
}

$sub_session = isset($session) ? $session : (isset($_GET['session']) ? $_GET['session'] : '');
$sub_self = (isset($_GET['hotspot']) && $_GET['hotspot'] == 'subscription')
          ? './?hotspot=subscription' . ($sub_session != '' ? '&session=' . urlencode($sub_session) : '')
          : './admin.php?id=subscription' . ($sub_session != '' ? '&session=' . urlencode($sub_session) : '');

/* ---------------------------------------------------------------- aksi */
/* Satu-satunya aksi di halaman ini: minta portal diperiksa sekarang.
 * Tidak mengubah data apa pun, jadi tidak perlu token sesi. */
if (isset($_POST['sub_action']) && $_POST['sub_action'] == 'refresh') {
  mikhmon_heartbeat_refresh(true);
  $sub_now = mikhmon_license_status();
  if (!$sub_now['configured']) {
    $_SESSION['mikhmon_sub_flash'] = 'err:Panel belum terhubung ke portal. Minta file include/instance.php ke NOCIFY.';
  } else {
    $_SESSION['mikhmon_sub_flash'] = !empty($sub_now['stale'])
      ? 'err:Portal belum bisa dihubungi. Data terakhir tetap dipakai.'
      : 'ok:Data langganan berhasil diperiksa.';
  }
  echo "<script>window.location='" . $sub_self . "'</script>";
  return;
}

/* Pesan sekali tampil. */
$sub_flash = '';
if (isset($_SESSION['mikhmon_sub_flash']) && $_SESSION['mikhmon_sub_flash'] != '') {
  $sub_flash = $_SESSION['mikhmon_sub_flash'];
  $_SESSION['mikhmon_sub_flash'] = '';
}

if ($sub_flash != '') {
  list($sub_kind, $sub_text) = explode(':', $sub_flash, 2);
} else {
  $sub_kind = '';
  $sub_text = '';
}

$sub = mikhmon_license_status();
$sub_locked = mikhmon_license_locked();

$sub_state_label = array(
  'unconfigured' => array('Belum Terhubung', 'bg-info'),
  'active'       => array('Aktif', 'bg-success'),
  'warning'      => array('Segera Berakhir', 'bg-warning'),
  'grace'        => array('Masa Tenggang', 'bg-warning'),
  'expired'      => array('Berakhir', 'bg-danger'),
);
$label = isset($sub_state_label[$sub['state']])
       ? $sub_state_label[$sub['state']]
       : array(htmlspecialchars($sub['state'], ENT_QUOTES), 'bg-info');

$sub_pretty_id = ($sub['install_id'] != '') ? mikhmon_license_pretty_id($sub['install_id']) : '';

/* Portal tujuan: tautan pembayaran kalau ada, kalau tidak akar portalnya. */
$sub_pay_url = ($sub['pay_url'] != '') ? $sub['pay_url'] : $sub['portal'];

/* Teks WhatsApp untuk tanya / perpanjang langganan. */
$sub_wa_text = "Halo NOCIFY, saya mau tanya soal langganan Mikhmon.\n\n";
if ($sub_pretty_id != '') {
  $sub_wa_text .= "ID Instalasi : " . $sub_pretty_id . "\n";
}
if ($sub['plan_label'] != '') {
  $sub_wa_text .= "Paket : " . $sub['plan_label'] . "\n";
}
if ($sub['expires'] != '') {
  $sub_wa_text .= "Berlaku sampai : " . $sub['expires'] . "\n";
}
if ($sub['state'] == 'expired') {
  $sub_wa_text .= "\nLangganan saya berakhir. Mohon dibantu perpanjangan. Terima kasih.";
} else {
  $sub_wa_text .= "\nMohon info perpanjangan langganan. Terima kasih.";
}
?>

<?php if ($sub_locked) { ?>
<div class="row">
  <div class="col-12">
    <div class="box bg-danger" style="text-align:center;padding:25px">
      <i class="fa fa-lock" style="font-size:42px"></i>
      <h2 style="margin:8px 0">LANGGANAN BERAKHIR</h2>
      <p style="margin:0">
        <?= htmlspecialchars($sub['message'], ENT_QUOTES) ?>
      </p>
      <?php if ($sub['expires'] != '') { ?>
      <p style="margin:6px 0 0 0">
        Langganan terakhir berlaku sampai <b><?= htmlspecialchars($sub['expires'], ENT_QUOTES) ?></b>.
      </p>
      <?php } ?>
      <p style="margin:10px 0 0 0">
        Panel dikunci sampai langganan diperpanjang lewat portal.
        <?php if ($sub_pay_url != '') { ?>
        <a class="btn bg-primary" style="color:#fff" href="<?= htmlspecialchars($sub_pay_url, ENT_QUOTES) ?>"
           target="_blank" rel="noopener"><i class="fa fa-external-link"></i> Buka portal</a>
        <?php } ?>
      </p>
    </div>
  </div>
</div>
<?php } ?>

<?php if ($sub_text != '') { ?>
<div class="row">
  <div class="col-12">
    <div class="box <?= $sub_kind == 'ok' ? 'bg-success' : 'bg-danger' ?>">
      <i class="fa <?= $sub_kind == 'ok' ? 'fa-check' : 'fa-ban' ?>"></i>
      <?= htmlspecialchars($sub_text, ENT_QUOTES) ?>
    </div>
  </div>
</div>
<?php } ?>

<?php if (!$sub['configured']) { ?>
<div class="row">
  <div class="col-12">
    <div class="box bg-warning">
      <i class="fa fa-exclamation-triangle"></i>
      Panel ini belum terhubung ke portal langganan. Minta file <code>include/instance.php</code>
      ke NOCIFY, lalu salin ke folder <code>include/</code> di server ini.
    </div>
  </div>
</div>
<?php } ?>

<?php if ($sub['stale'] && $sub['configured']) { ?>
<div class="row">
  <div class="col-12">
    <div class="box bg-warning">
      <i class="fa fa-clock-o"></i>
      <?php if ($sub['checked_at'] !== null) { ?>
        Terakhir diperiksa <?= htmlspecialchars(date('d-m-Y H:i', $sub['checked_at']), ENT_QUOTES) ?>.
        Data mungkin sudah tidak terbaru.
      <?php } else { ?>
        Belum pernah berhasil diperiksa ke portal.
      <?php } ?>
    </div>
  </div>
</div>
<?php } ?>

<div class="row">
  <div class="col-12">
    <div class="card">
      <div class="card-header">
        <h3><i class="fa fa-credit-card"></i> Langganan Mikhmon</h3>
      </div>
      <div class="card-body">

        <table class="table" style="width:auto">
          <tr>
            <td style="padding:4px 16px 4px 0"><b>Status</b></td>
            <td style="padding:4px 0"><span class="box <?= $label[1] ?>" style="margin:0;padding:2px 10px"><?= $label[0] ?></span></td>
          </tr>
          <tr>
            <td style="padding:4px 16px 4px 0"><b>Paket</b></td>
            <td style="padding:4px 0"><?= $sub['plan_label'] != '' ? htmlspecialchars($sub['plan_label'], ENT_QUOTES) : '&mdash;' ?></td>
          </tr>
          <tr>
            <td style="padding:4px 16px 4px 0"><b>Berlaku sampai</b></td>
            <td style="padding:4px 0"><?= $sub['expires'] != '' ? htmlspecialchars($sub['expires'], ENT_QUOTES) : '&mdash;' ?></td>
          </tr>
          <tr>
            <td style="padding:4px 16px 4px 0"><b>Sisa waktu</b></td>
            <td style="padding:4px 0">
              <?php
                if ($sub['days'] === null) {
                  echo '&mdash;';
                } elseif ((int) $sub['days'] > 0) {
                  echo '<b>' . (int) $sub['days'] . ' hari lagi</b>';
                } elseif ((int) $sub['days'] == 0) {
                  echo '<b class="text-red">Berakhir hari ini</b>';
                } else {
                  echo '<b class="text-red">Lewat ' . abs((int) $sub['days']) . ' hari</b>';
                }
              ?>
            </td>
          </tr>
          <tr>
            <td style="padding:4px 16px 4px 0"><b>ID Instalasi</b></td>
            <td style="padding:4px 0">
              <?php if ($sub_pretty_id != '') { ?>
                <code id="subInstallId"><?= htmlspecialchars($sub_pretty_id, ENT_QUOTES) ?></code>
                <a href="javascript:void(0)" onclick="mikhmonCopyId(this)" title="Salin ID"><i class="fa fa-copy"></i></a>
                <div style="font-size:11px;opacity:.75">
                  Kirim ID ini ke NOCIFY kalau diminta. Portal memakai ID ini untuk mengenali instalasi Anda.
                </div>
              <?php } else { ?>
                &mdash;
                <div style="font-size:11px;opacity:.75">
                  ID instalasi belum ada karena panel belum terhubung ke portal.
                </div>
              <?php } ?>
            </td>
          </tr>
          <?php if ($sub['message'] != '') { ?>
          <tr>
            <td style="padding:4px 16px 4px 0"><b>Pesan portal</b></td>
            <td style="padding:4px 0"><?= htmlspecialchars($sub['message'], ENT_QUOTES) ?></td>
          </tr>
          <?php } ?>
          <?php if ($sub['checked_at'] !== null) { ?>
          <tr>
            <td style="padding:4px 16px 4px 0"><b>Terakhir diperiksa</b></td>
            <td style="padding:4px 0"><?= htmlspecialchars(date('d-m-Y H:i', $sub['checked_at']), ENT_QUOTES) ?></td>
          </tr>
          <?php } ?>
        </table>

        <p style="margin:14px 0 0 0">
          <?php if ($sub_pay_url != '') { ?>
          <a class="btn bg-primary" style="color:#fff"
             href="<?= htmlspecialchars($sub_pay_url, ENT_QUOTES) ?>" target="_blank" rel="noopener">
            <i class="fa fa-external-link"></i> Buka portal
          </a>
          <?php } ?>
          <a class="btn bg-success" style="color:#fff"
             href="<?= htmlspecialchars(mikhmon_wa_link($sub_wa_text), ENT_QUOTES) ?>" target="_blank" rel="noopener">
            <i class="fa fa-whatsapp"></i> Chat WhatsApp
          </a>
          <form method="post" action="<?= htmlspecialchars($sub_self, ENT_QUOTES) ?>" style="display:inline">
            <input type="hidden" name="sub_action" value="refresh">
            <button type="submit" class="btn bg-info" style="color:#fff">
              <i class="fa fa-refresh"></i> Periksa sekarang
            </button>
          </form>
        </p>

        <div style="margin-top:10px;font-size:12px;opacity:.75">
          Panel memeriksa status ke portal secara berkala. Pembayaran dan perpanjangan
          dilakukan di portal, bukan di halaman ini.
        </div>

      </div>
    </div>
  </div>
</div>

<script>
function mikhmonCopyId(el) {
  var txt = document.getElementById('subInstallId').innerHTML;
  var ta = document.createElement('textarea');
  ta.value = txt;
  document.body.appendChild(ta);
  ta.select();
  try { document.execCommand('copy'); } catch (e) {}
  document.body.removeChild(ta);
  el.innerHTML = '<i class="fa fa-check text-green"></i>';
}
</script>
