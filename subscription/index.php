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

$sub_error = '';

/* ---------------------------------------------------------------- aksi */
if (isset($_POST['sub_action'])) {

  if (!isset($_SESSION['mikhmon_sub_flash'])) {
    $_SESSION['mikhmon_sub_flash'] = '';
  }

  /* Aksi yang mengubah data wajib membawa token sesi, supaya halaman lain
   * tidak bisa memicu hapus lisensi / hapus QRIS lewat POST. */
  if (in_array($_POST['sub_action'], array('clear', 'qris-upload', 'qris-remove'))
      && !mikhmon_sub_token_ok()) {
    $_SESSION['mikhmon_sub_flash'] = 'err:Permintaan ditolak. Muat ulang halaman lalu coba lagi.';
    echo "<script>window.location='" . $sub_self . "'</script>";
    return;
  }

  if ($_POST['sub_action'] == 'activate') {
    $key = isset($_POST['license_key']) ? $_POST['license_key'] : '';
    list($ok, $res) = mikhmon_license_apply($key);
    if ($ok) {
      $_SESSION['mikhmon_sub_flash'] = 'ok:Lisensi berhasil dipasang. Berlaku sampai ' . $res['expires'] . '.';
    } else {
      $why = array(
        'format'        => 'Format kode salah. Contoh yang benar: MKN-20251231-P1M-A1B2C3D4',
        'paket'         => 'Nama paket pada kode tidak dikenal.',
        'tanda-tangan'  => 'Kode ini bukan untuk instalasi ini, atau salah ketik.',
        'gagal-tulis'   => 'Kode benar tapi file lisensi tidak bisa ditulis. Periksa izin tulis folder include/.',
      );
      $_SESSION['mikhmon_sub_flash'] = 'err:Kode ditolak. ' . (isset($why[$res]) ? $why[$res] : 'Kode tidak dikenal.');
    }
  } elseif ($_POST['sub_action'] == 'clear') {
    mikhmon_license_clear();
    $_SESSION['mikhmon_sub_flash'] = 'ok:Lisensi dihapus. Panel kembali ke mode tanpa lisensi.';
  } elseif ($_POST['sub_action'] == 'qris-upload') {
    $sub_error = mikhmon_qris_upload();
    $_SESSION['mikhmon_sub_flash'] = ($sub_error === '')
      ? 'ok:Gambar QRIS tersimpan.'
      : 'err:' . $sub_error;
  } elseif ($_POST['sub_action'] == 'qris-remove') {
    if (mikhmon_qris_remove()) {
      $_SESSION['mikhmon_sub_flash'] = 'ok:Gambar QRIS dihapus.';
    } else {
      $_SESSION['mikhmon_sub_flash'] = 'err:Tidak ada gambar QRIS yang bisa dihapus.';
    }
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
$sub_plans = mikhmon_license_plans();
$sub_qris = mikhmon_qris_image_url();

$sub_state_label = array(
  'trial'   => array('Tanpa Lisensi', 'bg-info'),
  'active'  => array('Aktif', 'bg-success'),
  'warning' => array('Segera Berakhir', 'bg-warning'),
  'grace'   => array('Masa Tenggang', 'bg-warning'),
  'expired' => array('Berakhir', 'bg-danger'),
);
$label = $sub_state_label[$sub['state']];

/* Teks WhatsApp untuk konfirmasi pembayaran. */
if ($sub['licensed']) {
  $sub_wa_text = "Halo NOCIFY, saya mau memperpanjang langganan Mikhmon.\n\n"
               . "ID Instalasi : " . mikhmon_license_pretty_id($sub['install_id']) . "\n"
               . "Berlaku sampai : " . $sub['expires'] . " (" . $sub['plan_label'] . ")\n\n"
               . "Saya sudah bayar lewat QRIS. Mohon dibantu aktivasi. Terima kasih.";
} else {
  $sub_wa_text = "Halo NOCIFY, saya mau berlangganan Mikhmon.\n\n"
               . "ID Instalasi : " . mikhmon_license_pretty_id($sub['install_id']) . "\n\n"
               . "Mohon info paket dan cara pembayaran lewat QRIS. Terima kasih.";
}
?>

<?php if ($sub_locked) { ?>
<div class="row">
  <div class="col-12">
    <div class="box bg-danger" style="text-align:center;padding:25px">
      <i class="fa fa-lock" style="font-size:42px"></i>
      <h2 style="margin:8px 0">LANGGANAN BERAKHIR</h2>
      <p style="margin:0">
        Masa langganan Mikhmon berakhir pada <b><?= htmlspecialchars($sub['expires'], ENT_QUOTES) ?></b>
        (<?= htmlspecialchars(abs((int) $sub['days']), ENT_QUOTES) ?> hari yang lalu).
      </p>
      <p style="margin:6px 0 0 0">
        Panel dikunci sampai lisensi diperpanjang. Silakan bayar lewat QRIS di bawah,
        lalu kirim bukti lewat WhatsApp untuk mengaktifkan kembali.
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
<?php if (!mikhmon_license_writable()) { ?>
<div class="row">
  <div class="col-12">
    <div class="box bg-danger">
      <i class="fa fa-exclamation-triangle"></i>
      Folder <code>include/</code> tidak bisa ditulis, jadi lisensi tidak akan tersimpan
      dan ID Instalasi di atas berubah setiap halaman dimuat. Perbaiki izin tulis
      folder <code>include/</code> di server dulu.
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
            <td style="padding:4px 0"><?= $sub['licensed'] ? htmlspecialchars($sub['plan_label'], ENT_QUOTES) : '&mdash;' ?></td>
          </tr>
          <tr>
            <td style="padding:4px 16px 4px 0"><b>Berlaku sampai</b></td>
            <td style="padding:4px 0"><?= $sub['licensed'] ? htmlspecialchars($sub['expires'], ENT_QUOTES) : '&mdash;' ?></td>
          </tr>
          <tr>
            <td style="padding:4px 16px 4px 0"><b>Sisa waktu</b></td>
            <td style="padding:4px 0">
              <?php
                if (!$sub['licensed']) {
                  echo '&mdash;';
                } elseif (!empty($sub['lifetime'])) {
                  echo 'Permanen';
                  echo (int) $sub['days'] . ' hari lagi';
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
              <code id="subInstallId"><?= htmlspecialchars(mikhmon_license_pretty_id($sub['install_id']), ENT_QUOTES) ?></code>
              <a href="javascript:void(0)" onclick="mikhmonCopyId(this)" title="Salin ID"><i class="fa fa-copy"></i></a>
              <div style="font-size:11px;opacity:.75">
                Kirim ID ini ke NOCIFY saat membeli lisensi. Kode lisensi hanya berlaku untuk ID ini.
              </div>
            </td>
          </tr>
          <?php if ($sub['licensed']) { ?>
          <tr>
            <td style="padding:4px 16px 4px 0"><b>Kode lisensi</b></td>
            <td style="padding:4px 0"><code><?= htmlspecialchars($sub['key_short'], ENT_QUOTES) ?></code></td>
          </tr>
          <?php } ?>
        </table>

      </div>
    </div>
  </div>
</div>

<div class="row">
  <div class="col-12">
    <div class="card">
      <div class="card-header">
        <h3><i class="fa fa-qrcode"></i> Pembayaran QRIS</h3>
      </div>
      <div class="card-body">
        <div class="row">
          <div class="col-6">
            <?php if ($sub_qris != '') { ?>
              <img src="<?= $sub_qris ?>" alt="QRIS <?= htmlspecialchars(MIKHMON_QRIS_MERCHANT, ENT_QUOTES) ?>"
                   style="max-width:320px;width:100%;border-radius:6px;background:#fff;padding:6px">
            <?php } else { ?>
              <div class="box bg-warning">
                <i class="fa fa-exclamation-triangle"></i>
                Gambar QRIS belum diunggah. Unggah foto QRIS GoPay Merchant di bagian bawah halaman ini.
              </div>
            <?php } ?>
            <div style="margin-top:8px;font-size:13px">
              <div><b><?= htmlspecialchars(MIKHMON_QRIS_MERCHANT, ENT_QUOTES) ?></b></div>
              <div>NMID: <?= htmlspecialchars(MIKHMON_QRIS_NMID, ENT_QUOTES) ?></div>
              <div style="opacity:.75">GoPay Merchant &middot; QRIS</div>
            </div>
          </div>
          <div class="col-6">
            <p style="margin-top:0">
              Cara berlangganan:
            </p>
            <ol style="padding-left:18px">
              <li>Pilih paket di bawah, lalu scan gambar QRIS dengan aplikasi GoPay / bank apa pun.</li>
              <li>Masukkan nominal sesuai paket.</li>
              <li>Kirim bukti pembayaran lewat WhatsApp, sertakan ID Instalasi di atas.</li>
              <li>NOCIFY akan mengirim kode lisensi, lalu tempel di kolom Aktivasi Lisensi.</li>
            </ol>

            <table class="table" style="width:auto">
              <?php foreach ($sub_plans as $p) { if (empty($p['sale'])) continue; ?>
              <tr>
                <td style="padding:4px 16px 4px 0"><?= htmlspecialchars($p['label'], ENT_QUOTES) ?></td>
                <td style="padding:4px 0">
                  <?php if ($p['price'] > 0) { ?>
                    <b><?= mikhmon_license_price($p['price']) ?></b>
                  <?php } else { ?>
                    <i>Hubungi Admin</i>
                  <?php } ?>
                </td>
              </tr>
              <?php } ?>
            </table>

            <a class="btn bg-success" style="color:#fff"
               href="<?= htmlspecialchars(mikhmon_wa_link($sub_wa_text), ENT_QUOTES) ?>" target="_blank" rel="noopener">
              <i class="fa fa-whatsapp"></i> Konfirmasi Pembayaran via WhatsApp
            </a>
            <a class="btn bg-info" style="color:#fff"
               href="<?= htmlspecialchars(mikhmon_wa_link("Halo NOCIFY, saya mau tanya soal langganan Mikhmon.\n\nID Instalasi : " . mikhmon_license_pretty_id($sub['install_id'])), ENT_QUOTES) ?>"
               target="_blank" rel="noopener">
              <i class="fa fa-whatsapp"></i> Tanya Admin
            </a>
          </div>
        </div>
      </div>
    </div>
  </div>
</div>

<div class="row">
  <div class="col-12">
    <div class="card">
      <div class="card-header">
        <h3><i class="fa fa-key"></i> Aktivasi Lisensi</h3>
      </div>
      <div class="card-body">
        <p style="margin-top:0">
          Tempel kode lisensi yang dikirim NOCIFY. Kode berbentuk
          <code>MKN-YYYYMMDD-PAKET-XXXXXXXX</code>.
        </p>
        <form method="post" action="<?= htmlspecialchars($sub_self, ENT_QUOTES) ?>" autocomplete="off">
          <input type="hidden" name="sub_action" value="activate">
          <input type="text" name="license_key" class="group-item" placeholder="MKN-20251231-P1M-A1B2C3D4"
                 style="min-width:340px;text-transform:uppercase" required>
          <button type="submit" class="btn bg-primary"><i class="fa fa-check"></i> Aktifkan</button>
        </form>

        <?php if ($sub['licensed']) { ?>
        <form method="post" action="<?= htmlspecialchars($sub_self, ENT_QUOTES) ?>"
              onsubmit="return confirm('Hapus lisensi dari instalasi ini?')">
          <input type="hidden" name="sub_action" value="clear">
          <input type="hidden" name="sub_token" value="<?= htmlspecialchars(mikhmon_sub_token(), ENT_QUOTES) ?>">
          <button type="submit" class="btn bg-danger" style="color:#fff"><i class="fa fa-trash"></i> Hapus Lisensi</button>
        </form>
        <?php } ?>

        <details style="margin-top:14px">
          <summary style="cursor:pointer">Gambar QRIS merchant</summary>
          <form method="post" action="<?= htmlspecialchars($sub_self, ENT_QUOTES) ?>" enctype="multipart/form-data" style="margin-top:8px">
            <input type="hidden" name="sub_action" value="qris-upload">
            <input type="hidden" name="sub_token" value="<?= htmlspecialchars(mikhmon_sub_token(), ENT_QUOTES) ?>">
            <input type="file" name="QRIS" accept="image/png,image/jpeg,image/webp" required>
            <button type="submit" class="btn bg-primary"><i class="fa fa-upload"></i> Unggah QRIS</button>
          </form>
          <?php if ($sub_qris != '') { ?>
          <form method="post" action="<?= htmlspecialchars($sub_self, ENT_QUOTES) ?>"
                onsubmit="return confirm('Hapus gambar QRIS?')">
            <input type="hidden" name="sub_action" value="qris-remove">
            <input type="hidden" name="sub_token" value="<?= htmlspecialchars(mikhmon_sub_token(), ENT_QUOTES) ?>">
            <button type="submit" class="btn bg-danger" style="color:#fff"><i class="fa fa-trash"></i> Hapus gambar QRIS</button>
          </form>
          <?php } ?>
          <div style="font-size:12px;opacity:.75">
            Format PNG / JPG / WEBP, maksimal 2 MB. Tersimpan sebagai <code>img/qris-merchant.*</code>.
          </div>
        </details>
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
