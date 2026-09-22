<?php 
if(substr($_SERVER["REQUEST_URI"], -10) == "config.php"){header("Location:./");}; 
$data['mikhmon'] = array ('1'=>'mikhmon<|<mikhmon','mikhmon>|>aWNlbA==');

/*
 * Kredensial router TIDAK disimpan di berkas ini, tetapi satu berkas per sesi
 * di include/sessions/<nama>.php. Tujuannya supaya permintaan milik satu
 * pelanggan tidak pernah memuat password router pelanggan lain.
 *
 * mikhmon_config_boot() memuat hanya sesi yang benar-benar dipakai:
 *   - permintaan dari <nama>.nocify.id -> hanya sesi pelanggan itu,
 *   - ada ?session=<nama> dan berkasnya ada -> hanya sesi itu,
 *   - selain itu (daftar router di admin, localhost) -> semua sesi.
 *
 * Instalasi yang belum dipindahkan (include/sessions/ masih kosong) tetap
 * jalan: baris lama di berkas ini dibaca seperti sebelumnya.
 */
include_once(dirname(__FILE__) . '/sessions.php');
mikhmon_config_boot(isset($session) ? $session : '');
