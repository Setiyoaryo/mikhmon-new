<?php
/*
 *  Mikhmon Nocify - contoh kredensial instalasi dari portal langganan.
 *
 *  THIS PROGRAM IS FREE SOFTWARE; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation; either version 2 of the License, or
 *  (at your option) any later version.
 *
 *  Copy file ini menjadi include/instance.php, lalu isi nilainya dengan yang
 *  tertera di portal langganan NOCIFY (menu Instalasi / Pelanggan). File
 *  include/instance.php TIDAK ikut di-commit karena berisi token rahasia
 *  milik instalasi ini - kalau token itu bocor, orang lain bisa memakai
 *  langganan Anda.
 *
 *  Nilai 'portal' adalah alamat portal, tanpa garis miring di akhir.
 */

$mikhmon_instance = array(
  'id'     => 'CONTOH000000',
  'token'  => 'ganti-dengan-token-dari-portal',
  'portal' => 'https://portal.contoh.id',
);
