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
 *  along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
session_start();
// hide all error
error_reporting(0);
if (!isset($_SESSION["mikhmon"])) {
  header("Location:../admin.php?id=login");
} else {
}

/* Langganan: dipakai untuk baris status dan tautan WhatsApp di bawah. */
include_once(dirname(__FILE__) . '/subscription.php');
?>
<style>
.iFWrapper {
	position: relative;
	padding-bottom: 56.25%; /* 16:9 */
	padding-top: 25px;
	height: 0;
}
.iFWrapper iframe {
	position: absolute;
	top: 0;
	left: 0;
	width: 100%;
  height: 100%;
  border :none;
}
</style>
<div class="row">
  <div class="col-12">
    <div class="card">
      <div class="card-header">
        <h3><i class="fa fa-info-circle"></i> About</h3>
      </div>
      <div class="card-body">
        <h3>MIKHMON NOCIFY v<?= $_SESSION['v']; ?></h3>
<p>
  Ini adalah <b>hasil fork</b> dari MIKHMON V3. Tampilan dan alur kerjanya
  sengaja dibiarkan sama persis dengan aslinya, hanya bagian API/backend-nya
  yang ditulis ulang supaya lebih cepat dan sanggup menangani ribuan voucher.
  Versi ini dikembangkan, dipelihara, dan disewakan oleh <b>NOCIFY</b>.
</p>
<p>
  <ul>
    <li>
      <b>Asli &mdash; MIKHMON V3</b>
      <ul>
        <li>Author : Laksamadi Guko</li>
        <li>Licence : <a href="https://github.com/laksa19/mikhmonv2/blob/master/LICENSE">GPLv2</a></li>
        <li>API Class : <a href="https://github.com/BenMenking/routeros-api">routeros-api</a></li>
        <li>Website : <a href="https://laksa19.github.io">laksa19.github.io</a></li>
        <li>Facebook : <a href="https://fb.com/laksamadi">fb.com/laksamadi</a></li>
      </ul>
    </li>
    <li style="margin-top:8px">
      <b>Fork ini &mdash; NOCIFY</b>
      <ul>
        <li>Author : NOCIFY</li>
        <li>WhatsApp : <a href="<?= htmlspecialchars(mikhmon_wa_link("Halo NOCIFY, saya mau tanya soal Mikhmon."), ENT_QUOTES) ?>" target="_blank" rel="noopener">0851-3949-5106</a></li>
        <li>Backend : Go (rewrite), antarmuka tetap PHP seperti aslinya</li>
      </ul>
    </li>
  </ul>
</p>
<p>
  Terima kasih untuk Laksamadi Guko sebagai pembuat MIKHMON, dan untuk semua
  yang telah mendukung pengembangannya.
</p>
<div>
    <i>Copyright &copy; 2018 Laksamadi Guko &mdash; fork &copy; <?= date("Y") ?> NOCIFY</i>
</div>
<p style="margin-top:14px">
  <a class="btn bg-success" style="color:#fff"
     href="<?= htmlspecialchars(mikhmon_wa_link("Halo NOCIFY, saya mau tanya soal Mikhmon."), ENT_QUOTES) ?>"
     target="_blank" rel="noopener"><i class="fa fa-whatsapp"></i> Chat WhatsApp</a>
  <a class="btn bg-info" style="color:#fff" href="./admin.php?id=subscription"><i class="fa fa-credit-card"></i> Halaman Langganan</a>
</p>
<div class="box <?= mikhmon_license_status()['state'] == 'expired' ? 'bg-danger' : (mikhmon_license_warn() ? 'bg-warning' : 'bg-info') ?>" style="margin-left:0">
  <?php $sub_a = mikhmon_license_status(); ?>
  <?php if (!$sub_a['licensed']) { ?>
    Belum ada lisensi terpasang. <a href="./admin.php?id=subscription">Aktifkan langganan</a>.
  <?php } else { ?>
    Langganan <b><?= htmlspecialchars($sub_a['plan_label'], ENT_QUOTES) ?></b>
    berlaku sampai <b><?= htmlspecialchars($sub_a['expires'], ENT_QUOTES) ?></b>
    <?php
      if ((int) $sub_a['days'] >= 0) {
        echo '(' . (int) $sub_a['days'] . ' hari lagi)';
      } else {
        echo '&mdash; <b>sudah berakhir ' . abs((int) $sub_a['days']) . ' hari lalu</b>';
      }
    ?>.
    <a href="./admin.php?id=subscription">Perpanjang</a>.
  <?php } ?>
</div>
</div>
</div>
</div>
</div>
<div class="col-12">
<div class="card">
  <div class="card-header">
  <h3><i class="fa fa-info-circle"></i> Changelog</h3>
  </div>
  <div class="card-body">
  <div class="iFWrapper">
    <iframe src="https://laksa19.github.io/mikhmonv3" ></iframe>
  </div>
  </div>
</div>
</div>
</div>
