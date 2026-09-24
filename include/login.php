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


$t_name = function_exists('mikhmon_tenant_session') ? mikhmon_tenant_session() : '';
$t_brand = 'MIKHMON';
$t_logo = 'img/favicon.png';

if ($t_name !== '') {
  if (function_exists('mikhmon_tenant_file')) {
    if (mikhmon_tenant_file('logo.png', $t_name) !== '') {
      $t_logo = 'tenant-logo.png';
    } elseif (mikhmon_tenant_file('logo.jpg', $t_name) !== '') {
      $t_logo = 'tenant-logo.jpg';
    }
    $brandFile = mikhmon_tenant_file('brand.txt', $t_name);
    if ($brandFile !== '') {
      $t_brand = htmlspecialchars(trim(@file_get_contents($brandFile)), ENT_QUOTES);
    } elseif (!empty($hotspotname)) {
      $t_brand = htmlspecialchars($hotspotname, ENT_QUOTES);
    } else {
      $t_brand = ucfirst($t_name) . ' Hotspot';
    }
  }
}
?>

<div style="padding-top: 5%;"  class="login-box">
  <div class="card">
    <div class="card-header">
      <h3><?= $_please_login ?></h3>
    </div>
    <div class="card-body">
      <div class="text-center pd-5">
        <img src="<?= $t_logo; ?>" alt="<?= $t_brand; ?> Logo" style="max-height:80px; max-width:180px;">
      </div>
      <div  class="text-center">
      <span style="font-size: 25px; margin: 10px;"><?= $t_brand; ?></span>
      </div>
      <center>
      <form autocomplete="off" action="" method="post">
      <table class="table" style="width:90%">
        <tr>
          <td class="align-middle text-center">
            <input style="width: 100%; height: 35px; font-size: 16px;" class="form-control" type="text" name="user" id="_username" placeholder="Username" required="1" autofocus>
          </td>
        </tr>
        <tr>
          <td class="align-middle text-center">
            <input style="width: 100%; height: 35px; font-size: 16px;" class="form-control" type="password" name="pass" placeholder="Password" required="1">
          </td>
        </tr>
        <tr>
          <td class="align-middle text-center">
            <input style="width: 100%; margin-top:20px; height: 35px; font-weight: bold; font-size: 17px;" class="btn-login bg-primary pointer" type="submit" name="login" value="Login">
          </td>
        </tr>
        <tr>
          <td class="align-middle text-center">
            <?= $error; ?>
          </td>
        </tr>
      </table>
      </form>
      </center>
    </div>
  </div>
</div>

</body>
</html>
