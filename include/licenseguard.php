<?php
/*
 *  Mikhmon Nocify - gerbang langganan untuk endpoint yang dipanggil langsung.
 *
 *  Halaman utama sudah dijaga oleh index.php dan admin.php. File ini dipakai
 *  oleh endpoint yang masih bisa diakses tanpa lewat keduanya, misalnya
 *  dashboard/aload.php (dipanggil berulang oleh dashboard), hotspotactive.php,
 *  livereport.php, dan halaman cetak voucher.
 *
 *  Copyright (C) 2024 NOCIFY.  GPLv2.
 */

if (!defined('MIKHMON_SUBSCRIPTION_LOADED')) {
  include_once(dirname(__FILE__) . '/subscription.php');
}

if (function_exists('mikhmon_license_locked') && mikhmon_license_locked()) {
  $mikhmon_gate = isset($_GET['id']) ? $_GET['id'] : (isset($_GET['hotspot']) ? $_GET['hotspot'] : '');
  if ($mikhmon_gate != 'subscription' && $mikhmon_gate != 'logout' && $mikhmon_gate != 'login') {
    header('HTTP/1.1 403 Forbidden');
    header('Content-Type: text/plain; charset=utf-8');
    echo "Langganan Mikhmon berakhir. Buka menu Langganan untuk memperpanjang.";
    exit;
  }
}
