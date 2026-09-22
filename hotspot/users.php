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

// hide all error
error_reporting(0);
ini_set('max_execution_time', 300);

if (!isset($_SESSION["mikhmon"])) {
  header("Location:../admin.php?id=login");
} else {

  /*
   * Seluruh daftar user diambil sekali - dari simpanan sementara kalau masih
   * segar - lalu disaring di sini.
   *
   * Sebelumnya tiap filter diambil sendiri-sendiri dari router, dan tiap
   * pengambilan berarti router mengirim ulang SELURUH daftar. Di pemasangan
   * yang sudah berisi puluhan ribu user itu berarti beberapa detik untuk tiap
   * klik; halaman ini pun jadi menunggu lama hanya untuk berpindah halaman
   * atau mengganti filter.
   *
   * Menyaring di PHP menghasilkan daftar yang sama persis: saringan profile,
   * comment, dan limit-uptime di router hanya mencocokkan nilai yang sama pada
   * baris yang sama.
   */
  include_once(dirname(__FILE__) . '/../include/hscache.php');
  $semuauser = mikhmon_hscache_hotspot_users($API, $session);

  $exp = $_GET['exp'];
  if ($exp != "") {
    /* Hanya user yang masa pakainya sudah habis (limit-uptime 1s). */
    $getuser = array();
    foreach ($semuauser as $u) {
      if (isset($u['limit-uptime']) && $u['limit-uptime'] === '1s') {
        $getuser[] = $u;
      }
    }
  } elseif ($comm != "") {
    $getuser = array();
    foreach ($semuauser as $u) {
      if (isset($u['comment']) && $u['comment'] === $comm) {
        $getuser[] = $u;
      }
    }
  } elseif ($prof != "all") {
    $getuser = array();
    foreach ($semuauser as $u) {
      if (isset($u['profile']) && $u['profile'] === $prof) {
        $getuser[] = $u;
      }
    }
  } else {
    $getuser = $semuauser;
  }

  $TotalReg = count($getuser);
  $counttuser = $TotalReg;
  $getprofile = $API->comm("/ip/hotspot/user/profile/print");
  $TotalReg2 = count($getprofile);
}
?>

<div class="row">
<div class="col-12">
<div class="card">
<div class="card-header">
    <h3><i class="fa fa-users"></i> <?= $_users ?>
      <span style="font-size: 14px">
        <?php
        if ($counttuser == 0) {
          echo "<script>window.location='./?hotspot=users&profile=all&session=" . $session . "</script>";
        } ?>
         &nbsp; | &nbsp; <a href="./?hotspot-user=add&session=<?= $session; ?>" title="Add User"><i class="fa fa-user-plus"></i> <?= $_add ?></a>
        &nbsp; | &nbsp; <a href="./?hotspot-user=generate&session=<?= $session; ?>" title="Generate User"><i class="fa fa-users"></i> <?= $_generate ?></a>
         &nbsp; | &nbsp; <a href="<?= str_replace("=users", "=export-users", $url); ?>&export=script" title="Download User List as Mikrotik Script"><i class="fa fa-download"></i> Script</a>&nbsp; | &nbsp; <a href="<?= str_replace("=users", "=export-users", $url); ?>&export=csv" title="Download User List as CSV"><i class="fa fa-download"></i> CSV</a>
        </span>  &nbsp;
        <small id="loader" style="display: none;" ><i><i class='fa fa-circle-o-notch fa-spin'></i> <?= $_processing ?> </i></small>
    </h3>
    
</div>
<div class="card-body">
  <div class="row">
   <div class="col-6 pd-t-5 pd-b-5">
  <div class="input-group">
    <div class="input-group-4 col-box-4">
      <input id="filterTable" type="text" style="padding:5.8px;" class="group-item group-item-l" placeholder="<?= $_search ?>">
    </div>
    <div class="input-group-4 col-box-4">
      <select style="padding:5px;" class="group-item group-item-m" onchange="location = this.value; loader()" title="Filter by Profile">
        <option><?= $_profile ?> </option>
        <option value="./?hotspot=users&profile=all&session=<?= $session; ?>"><?= $_show_all ?></option>
      <?php
      for ($i = 0; $i < $TotalReg2; $i++) {
        $profile = $getprofile[$i];
        echo "<option value='./?hotspot=users&profile=" . $profile['name'] . "&session=" . $session . "'>" . $profile['name'] . "</option>";
      }
      ?>
    </select>
  </div>
  <div class="input-group-4 col-box-4">
    <select style="padding:5px;" class="group-item group-item-r" id="comment" name="comment" onchange="location = './?hotspot=users&comment='+ this.value +'&session=<?= $session;?>';">
    <?php
    if ($comm != "") {
    } else {
      echo "<option value=''>".$_comment."</option>";
    }
    $TotalReg = count($getuser);
    for ($i = 0; $i < $TotalReg; $i++) {
      $ucomment = $getuser[$i]['comment'];
      $uprofile = $getuser[$i]['profile'];
      $acomment .= ",".$ucomment."#". $uprofile;
    }

    $ocomment=  explode(",",$acomment);
    
    $comments=array_count_values($ocomment) ;
    foreach ($comments as $tcomment=>$value) {

      if (is_numeric(substr($tcomment, 3, 3))) {
       
        echo "<option value='" . explode("#",$tcomment)[0] . "' >". explode("#",$tcomment)[0]." ".explode("#",$tcomment)[1]. " [".$value. "]</option>";
       }
 
    }

    ?>
    </select>
  </div>
  </div>
  </div>
 
  <div class="col-6">
    <?php if ($comm != "") { ?>
  <button class="btn bg-red" onclick="if(confirm('Are you sure to delete username by comment (<?= $comm; ?>)?')){loadpage('./?remove-hotspot-user-by-comment=<?= $comm; ?>&session=<?= $session; ?>');loader();}else{}" title="Remove user by comment <?= $comm; ?>">  <i class="fa fa-trash"></i> <?= $_by_comment ?></button>
    <?php ; }else if ($exp == "1"){ ?>
  <button class="btn bg-red" onclick="if(confirm('Are you sure to delete users?')){loadpage('./?remove-hotspot-user-expired=1&session=<?= $session; ?>');loader();}else{}" title="Remove user expired">  <i class="fa fa-trash"></i> Expired Users</button>
      <?php } ?>
  <?php // Retention cleanup: deletes vouchers that were never used and whose
        // batch comment is older than 30 days. Shown always, because it is not
        // tied to the current filter the way the two buttons above are. ?>
  <button class="btn bg-red" onclick="if(confirm('Remove UNUSED vouchers older than 30 days?\n\nOnly vouchers that have never been logged in with (uptime 0s) and whose comment date is older than 30 days are removed. Vouchers that were used are not touched.')){loadpage('./?remove-unused-hotspot-user=1&days=30&session=<?= $session; ?>');loader();}else{}" title="Remove unused vouchers older than 30 days">  <i class="fa fa-trash"></i> <?= $_unused_older ?></button>
  <script>
    function printV(a,b){
    var comm = document.getElementById('comment').value;
    var url = "./voucher/print.php?id="+comm+"&"+a+"="+b+"&session=<?= $session; ?>";
    if (comm === "" ){
      <?php if ($currency == in_array($currency, $cekindo['indo'])) { ?>
      alert('Silakan pilih salah satu Comment terlebih dulu!');
      <?php
    } else { ?>
      alert('Please choose one of the Comments first!');
      <?php
    } ?>
    }else{
      var win = window.open(url, '_blank');
      win.focus();
    }}
  </script>
  <button class="btn bg-primary" title='Print' onclick="printV('qr','no');"><i class="fa fa-print"></i> <?= $_print_default ?></button>
  <button class="btn bg-primary" title='Print QR' onclick="printV('qr','yes');"><i class="fa fa-print"></i> <?= $_print_qr ?></button>
  <button class="btn bg-primary" title='Print Small'onclick="printV('small','yes');"><i class="fa fa-print"></i> <?= $_print_small ?></button>
  </div>
</div>
<div class="overflow mr-t-10 box-bordered" style="max-height: 75vh">
<table id="dataTable" class="table table-bordered table-hover text-nowrap">
  <thead>
  <tr>
    <th style="min-width:50px;" class="align-middle text-center" id="cuser"><?= $counttuser; ?></th>
    <th style="min-width:50px;" class="pointer" title="Click to sort"><i class="fa fa-sort"></i> Server</th>
    <th class="pointer" title="Click to sort"><i class="fa fa-sort"></i> <?= $_name ?></th>
    <th>Print</th>
    <th class="pointer" title="Click to sort"><i class="fa fa-sort"></i> <?= $_profile ?></th>
	  <th class="pointer" title="Click to sort"><i class="fa fa-sort"></i> Mac Address</th>
    <th class="text-right align-middle pointer" title="Click to sort"><i class="fa fa-sort"></i> <?= $_uptime_user ?></th>
    <th class="text-right align-middle pointer" title="Click to sort"><i class="fa fa-sort"></i> Bytes In</th>
    <th class="text-right align-middle pointer" title="Click to sort"><i class="fa fa-sort"></i> Bytes Out</th>
    <th class="pointer" title="Click to sort"><i class="fa fa-sort"></i> <?= $_comment ?></th>
    </tr>
  </thead>
  <tbody id="tbody">
<?php
// Only render one page of rows. The browser keeps every row it is given and
// each one costs about 22 elements, so handing it a router with thousands of
// users makes the tab allocate gigabytes and stutter while scrolling.
$pf_per = isset($_GET['per']) ? (int) $_GET['per'] : 100;
if ($pf_per < 10 || $pf_per > 1000) {
  $pf_per = 100;
}
$pf_pages = $TotalReg > 0 ? (int) ceil($TotalReg / $pf_per) : 1;
$pf_page = isset($_GET['page']) ? (int) $_GET['page'] : 1;
if ($pf_page < 1) {
  $pf_page = 1;
}
if ($pf_page > $pf_pages) {
  $pf_page = $pf_pages;
}
$pf_offset = ($pf_page - 1) * $pf_per;
$pf_end = min($pf_offset + $pf_per, $TotalReg);

// Page links keep whichever filter brought the user here.
$pf_base = "./?hotspot=users&session=" . $session;
if ($prof != "") {
  $pf_base .= "&profile=" . urlencode($prof);
}
if ($comm != "") {
  $pf_base .= "&comment=" . urlencode($comm);
}
if ($exp == "1") {
  $pf_base .= "&exp=1";
}
$pf_base .= "&per=" . $pf_per;

for ($i = $pf_offset; $i < $pf_end; $i++) {
  $userdetails = $getuser[$i];
  $uid = $userdetails['.id'];
  $userver = $userdetails['server'];
  $uname = $userdetails['name'];
  $upass = $userdetails['password'];
  $uprofile = $userdetails['profile'];
  $umacadd = $userdetails['mac-address'];
  $uuptime = formatDTM($userdetails['uptime']);
  $ubytesi = formatBytes($userdetails['bytes-in'], 2);
  $ubyteso = formatBytes($userdetails['bytes-out'], 2);

  $ucomment = $userdetails['comment'];
  $udisabled = $userdetails['disabled'];
  $utimelimit = $userdetails['limit-uptime'];
  if ($utimelimit == '1s') {
    $utimelimit = ' expired';
  } else {
    $utimelimit = ' ' . $utimelimit;
  }
  $udatalimit = $userdetails['limit-bytes-total'];
  if ($udatalimit == '') {
    $udatalimit = '';
  } else {
    $udatalimit = ' ' . formatBytes($udatalimit, 2);
  }

  echo "<tr>";
  ?>
  <td style='text-align:center;'>  <i class='fa fa-minus-square text-danger pointer' onclick="if(confirm('Are you sure to delete username (<?= $uname; ?>)?')){loadpage('./?remove-hotspot-user=<?= $uid; ?>&session=<?= $session; ?>')}else{}" title='Remove <?= $uname; ?>'></i>&nbsp&nbsp&nbsp&nbsp&nbsp&nbsp
  <?php
  if ($udisabled == "true") {
    $uriprocess = "'./?enable-hotspot-user=" . $uid . "&session=" . $session."'";
    echo '<span class="text-warning pointer" title="Enable User ' . $uname . '"  onclick="loadpage('.$uriprocess.')"><i class="fa fa-lock "></i></span></td>';
  } else {
    $uriprocess = "'./?disable-hotspot-user=" . $uid . "&session=" . $session."'";
    echo '<span class="pointer" title="Disable User ' . $uname . '"  onclick="loadpage('.$uriprocess.')"><i class="fa fa-unlock "></i></span></td>';
  }
  echo "<td>" . $userver . "</td>";
  if ($uname == $upass) {
    $usermode = "vc";
  } else {
    $usermode = "up";
  }
  $popup = "javascript:window.open('./voucher/print.php?user=" . $usermode . "-" . $uname . "&qr=no&session=" . $session . "','_blank','width=320,height=550').print();";
  $popupQR = "javascript:window.open('./voucher/print.php?user=" . $usermode . "-" . $uname . "&qr=yes&session=" . $session . "','_blank','width=320,height=550').print();";
  echo "<td><a title='Open User " . $uname . "' href=./?hotspot-user=" . $uid . "&session=" . $session . "><i class='fa fa-edit'></i> " . $uname . " </a>";
  echo '</td><td class"text-center"><a title="Print ' . $uname . '" href="' . $popup . '"><i class="fa fa-print"></i></a> &nbsp <a title="Print ' . $uname . '" href="' . $popupQR . '"><i class="fa fa-qrcode"></i> </a></td>';
  echo "<td>" . $uprofile . "</td>";
  echo "<td style=' text-align:left'>" . $umacadd . "</td>";
  echo "<td style=' text-align:right'>" . $uuptime . "</td>";
  echo "<td style=' text-align:right'>" . $ubytesi . "</td>";
  echo "<td style=' text-align:right'>" . $ubyteso . "</td>";
  echo "<td>";
  if ($uname == "default-trial") {
  } else if (substr($ucomment,0,3) == "vc-" || substr($ucomment,0,3) == "up-") {
    echo "<a href=./?hotspot=users&comment=" . $ucomment . "&session=" . $session . " title='Filter by " . $ucomment . "'><i class='fa fa-search'></i> ". $ucomment." ". $udatalimit ." ".$utimelimit . "</a>";
  } else if ($utimelimit == ' expired') {
    echo "<a href=./?hotspot=users&profile=all&exp=1&session=" . $session . " title='Filter by expired'><i class='fa fa-search'></i> " . $ucomment." ". $udatalimit ." ".$utimelimit . "</a>";
  }else{
    echo $ucomment.' ';
  }
  echo  "</td>";


}
?>
  </tr>
</table>
<?php if ($pf_pages > 1) { ?>
<div class="row" style="margin-top:10px;">
  <div class="col-12 box-group" style="padding:8px 12px;">
    <span class="pd-2p5"><?= $pf_offset + 1 ?>-<?= $pf_end ?> / <?= $TotalReg ?></span>
    <?php if ($pf_page > 1) { ?>
      <a class="btn bg-secondary" title="First" href="<?= $pf_base ?>&page=1"><i class="fa fa-angle-double-left"></i></a>
      <a class="btn bg-secondary" title="Previous" href="<?= $pf_base ?>&page=<?= $pf_page - 1 ?>"><i class="fa fa-angle-left"></i></a>
    <?php } ?>
    <span class="pd-2p5"><?= $pf_page ?> / <?= $pf_pages ?></span>
    <?php if ($pf_page < $pf_pages) { ?>
      <a class="btn bg-secondary" title="Next" href="<?= $pf_base ?>&page=<?= $pf_page + 1 ?>"><i class="fa fa-angle-right"></i></a>
      <a class="btn bg-secondary" title="Last" href="<?= $pf_base ?>&page=<?= $pf_pages ?>"><i class="fa fa-angle-double-right"></i></a>
    <?php } ?>
  </div>
</div>
<?php } ?>
</div>
</div>
</div>
</div>
</div>

	
	
