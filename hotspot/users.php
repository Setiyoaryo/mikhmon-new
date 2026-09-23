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
  if (!is_array($semuauser)) {
    $semuauser = array();
  }

  $exp = isset($_GET['exp']) ? $_GET['exp'] : '';
  $status = isset($_GET['status']) ? trim($_GET['status']) : '';
  if ($exp == "1" && $status == "") {
    $status = "expired";
  }

  $user_status_fn = function ($u) {
    $limituptime = isset($u['limit-uptime']) ? (string)$u['limit-uptime'] : '';
    $uptime = isset($u['uptime']) ? (string)$u['uptime'] : '';
    $comment = isset($u['comment']) ? (string)$u['comment'] : '';
    $bytesin = isset($u['bytes-in']) ? (int)$u['bytes-in'] : 0;
    $bytesout = isset($u['bytes-out']) ? (int)$u['bytes-out'] : 0;
    $profile = isset($u['profile']) ? strtolower((string)$u['profile']) : '';
    $name = isset($u['name']) ? (string)$u['name'] : '';
    $pass = isset($u['password']) ? (string)$u['password'] : '';

    if ($limituptime === '1s') {
      return 'expired';
    }

    // Uptime limit exhausted (e.g. limit-uptime reached)
    if ($limituptime !== '' && $uptime !== '' && $limituptime === $uptime && $limituptime !== '0s') {
      return 'expired';
    }

    $isBatchVc = (substr($comment, 0, 3) === 'vc-' || substr($comment, 0, 3) === 'up-');
    $isExpComment = false;
    $expTime = 0;
    if (preg_match('/^(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec|\d{2})\/\d{2}/i', $comment)) {
      $isExpComment = true;
      $parsed = strtotime(str_replace('/', ' ', $comment));
      if ($parsed !== false) {
        $expTime = $parsed;
      }
    }

    if ($expTime > 0 && $expTime <= time()) {
      return 'expired';
    }

    if (strpos($profile, 'member') !== false || strpos(strtolower($comment), 'member') !== false) {
      return 'member';
    }

    if (!$isBatchVc && !$isExpComment && $name !== 'default-trial') {
      if ($name !== $pass || (strpos($comment, 'vc-') === false && strpos($comment, 'up-') === false && $comment !== '')) {
        return 'member';
      }
    }

    $hasUptime = ($uptime !== '' && $uptime !== '0s' && $uptime !== '00:00:00');
    $hasTraffic = ($bytesin > 0 || $bytesout > 0);
    if ($hasUptime || $hasTraffic || ($expTime > time()) || $isExpComment) {
      return 'active';
    }

    return 'ready';
  };

  // Base list filtered by profile or comment
  if ($comm != "") {
    $base_list = array();
    foreach ($semuauser as $u) {
      if (isset($u['comment']) && $u['comment'] === $comm) {
        $base_list[] = $u;
      }
    }
  } elseif ($prof != "all" && $prof != "") {
    $base_list = array();
    foreach ($semuauser as $u) {
      if (isset($u['profile']) && $u['profile'] === $prof) {
        $base_list[] = $u;
      }
    }
  } else {
    $base_list = $semuauser;
  }

  // Count status tabs on base list
  $status_counts = array(
    'all'     => count($base_list),
    'ready'   => 0,
    'active'  => 0,
    'expired' => 0,
    'member'  => 0,
  );
  foreach ($base_list as $u) {
    $st = $user_status_fn($u);
    if (isset($status_counts[$st])) {
      $status_counts[$st]++;
    }
  }

  // Filter getuser by selected status tab
  if ($status != "" && $status != "all") {
    $getuser = array();
    foreach ($base_list as $u) {
      if ($user_status_fn($u) === $status) {
        $getuser[] = $u;
      }
    }
  } else {
    $getuser = $base_list;
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
        if ($prof != "all" && $prof != "" && $comm == "" && $status == "" && $counttuser == 0 && count($semuauser) > 0) {
          echo "<script>window.location='./?hotspot=users&profile=all&session=" . $session . "';</script>";
        } ?>
         &nbsp; | &nbsp; <a href="./?hotspot-user=add&session=<?= $session; ?>" title="Add User"><i class="fa fa-user-plus"></i> <?= $_add ?></a>
        &nbsp; | &nbsp; <a href="./?hotspot-user=generate&session=<?= $session; ?>" title="Generate User"><i class="fa fa-users"></i> <?= $_generate ?></a>
         &nbsp; | &nbsp; <a href="<?= str_replace("=users", "=export-users", $url); ?>&export=script" title="Download User List as Mikrotik Script"><i class="fa fa-download"></i> Script</a>&nbsp; | &nbsp; <a href="<?= str_replace("=users", "=export-users", $url); ?>&export=csv" title="Download User List as CSV"><i class="fa fa-download"></i> CSV</a>
        </span>  &nbsp;
        <small id="loader" style="display: none;" ><i><i class='fa fa-circle-o-notch fa-spin'></i> <?= $_processing ?> </i></small>
    </h3>
    
</div>
<?php
/*
 * Keterangan sesaat setelah voucher dibuat: berapa yang jadi, batchnya yang
 * mana, dan bahwa tombol Print di toolbar mencetak seluruh batch itu. Dulu
 * halaman Generate langsung kembali ke form kosong, jadi tidak ada satu pun
 * tanda bahwa 1000 voucher sudah jadi - dan tidak jelas apa yang akan dicetak.
 */
if (isset($_SESSION['mikhmon_generate_hasil']) && is_array($_SESSION['mikhmon_generate_hasil'])) {
  $gh = $_SESSION['mikhmon_generate_hasil'];
  unset($_SESSION['mikhmon_generate_hasil']);
  $gh_qty = isset($gh['added']) ? (int) $gh['added'] : 0;
  $gh_gagal = isset($gh['failed']) ? (int) $gh['failed'] : 0;
  /* Kunci batch: komentar yang sama dipakai halaman cetak untuk mengenali batch
   * ini, jadi tombol cetak di bawah selalu mencetak batch yang baru dibuat. */
  $gh_kunci = isset($gh['comment']) ? (string) $gh['comment'] : '';
  ?>
<div class="card-body pd-b-0">
  <div class="box success">
    <b><?= number_format($gh_qty, 0, ",", "."); ?></b> voucher created for profile
    <b><?= htmlspecialchars(isset($gh['profile']) ? $gh['profile'] : '', ENT_QUOTES); ?></b>
    &middot; batch <span class="mono"><?= htmlspecialchars($gh_kunci, ENT_QUOTES); ?></span>
    <?php if ($gh_gagal > 0) { ?>
    &middot; <span class="cl-danger"><?= $gh_gagal; ?> failed</span>
    <?php } ?>
    <br>
    <span class="tiny">All of them are listed below - nothing else is mixed in.</span>
    <div class="pd-t-5">
      <a class="btn bg-primary" target="_blank" title="Print <?= $gh_qty; ?> vouchers"
         href="./voucher/print.php?id=<?= urlencode($gh_kunci); ?>&qr=no&session=<?= $session; ?>">
        <i class="fa fa-print"></i> <?= $_print; ?> <?= number_format($gh_qty, 0, ",", "."); ?></a>
      <a class="btn bg-danger" target="_blank" title="Print <?= $gh_qty; ?> vouchers with QR"
         href="./voucher/print.php?id=<?= urlencode($gh_kunci); ?>&qr=yes&session=<?= $session; ?>">
        <i class="fa fa-qrcode"></i> <?= $_print_qr; ?> <?= number_format($gh_qty, 0, ",", "."); ?></a>
      <a class="btn bg-info" target="_blank" title="Print <?= $gh_qty; ?> small vouchers"
         href="./voucher/print.php?id=<?= urlencode($gh_kunci); ?>&small=yes&session=<?= $session; ?>">
        <i class="fa fa-print"></i> <?= $_print_small; ?> <?= number_format($gh_qty, 0, ",", "."); ?></a>
    </div>
  </div>
</div>
  <?php
}
?>
<div class="card-body">
  <div class="row pd-b-5">
    <div class="col-12">
      <div style="display:flex; flex-wrap:wrap; gap:4px; align-items:center;">
        <?php
          $tab_base = "./?hotspot=users&session=" . $session;
          if ($prof != "" && $prof != "all") {
            $tab_base .= "&profile=" . urlencode($prof);
          }
          if ($comm != "") {
            $tab_base .= "&comment=" . urlencode($comm);
          }
          $is_all = ($status == "" || $status == "all");
        ?>
        <a href="<?= $tab_base; ?>&status=all" class="btn <?= $is_all ? 'bg-primary' : 'bg-secondary'; ?>" style="margin:2px;" title="<?= $_all; ?>">
          <i class="fa fa-users"></i> <?= $_all; ?> <span style="background:rgba(0,0,0,0.2); padding:1px 6px; border-radius:10px; font-size:12px; margin-left:3px;"><?= $status_counts['all']; ?></span>
        </a>
        <a href="<?= $tab_base; ?>&status=ready" class="btn <?= ($status == 'ready') ? 'bg-primary' : 'bg-secondary'; ?>" style="margin:2px;" title="<?= $_ready_stock; ?>">
          <i class="fa fa-ticket"></i> <?= $_ready_stock; ?> <span style="background:rgba(0,0,0,0.2); padding:1px 6px; border-radius:10px; font-size:12px; margin-left:3px;"><?= $status_counts['ready']; ?></span>
        </a>
        <a href="<?= $tab_base; ?>&status=active" class="btn <?= ($status == 'active') ? 'bg-primary' : 'bg-secondary'; ?>" style="margin:2px;" title="<?= $_in_use; ?>">
          <i class="fa fa-wifi"></i> <?= $_in_use; ?> <span style="background:rgba(0,0,0,0.2); padding:1px 6px; border-radius:10px; font-size:12px; margin-left:3px;"><?= $status_counts['active']; ?></span>
        </a>
        <a href="<?= $tab_base; ?>&status=expired" class="btn <?= ($status == 'expired') ? 'bg-primary' : 'bg-secondary'; ?>" style="margin:2px;" title="<?= $_expired; ?>">
          <i class="fa fa-clock-o"></i> <?= $_expired; ?> <span style="background:rgba(0,0,0,0.2); padding:1px 6px; border-radius:10px; font-size:12px; margin-left:3px;"><?= $status_counts['expired']; ?></span>
        </a>
        <a href="<?= $tab_base; ?>&status=member" class="btn <?= ($status == 'member') ? 'bg-primary' : 'bg-secondary'; ?>" style="margin:2px;" title="<?= $_member; ?>">
          <i class="fa fa-id-card-o"></i> <?= $_member; ?> <span style="background:rgba(0,0,0,0.2); padding:1px 6px; border-radius:10px; font-size:12px; margin-left:3px;"><?= $status_counts['member']; ?></span>
        </a>
      </div>
    </div>
  </div>
  <div class="row">
   <div class="col-6 pd-t-5 pd-b-5">
  <div class="input-group">
    <div class="input-group-4 col-box-4">
      <input id="filterTable" type="text" style="padding:5.8px;" class="group-item group-item-l" placeholder="<?= $_search ?>">
    </div>
    <div class="input-group-4 col-box-4">
      <select style="padding:5px;" class="group-item group-item-m" onchange="location = this.value; loader()" title="Filter by Profile">
        <option><?= $_profile ?> </option>
        <option value="./?hotspot=users&profile=all&session=<?= $session . (($status != '' && $status != 'all') ? '&status=' . urlencode($status) : ''); ?>"><?= $_show_all ?></option>
      <?php
      $st_param = ($status != '' && $status != 'all') ? '&status=' . urlencode($status) : '';
      for ($i = 0; $i < $TotalReg2; $i++) {
        $profile = $getprofile[$i];
        echo "<option value='./?hotspot=users&profile=" . $profile['name'] . "&session=" . $session . $st_param . "'>" . $profile['name'] . "</option>";
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
    $acomment = "";
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
    <?php ; }else if ($exp == "1" || $status == "expired"){ ?>
  <button class="btn bg-red" onclick="if(confirm('Are you sure to delete users?')){loadpage('./?remove-hotspot-user-expired=1&session=<?= $session; ?>');loader();}else{}" title="Remove user expired">  <i class="fa fa-trash"></i> Expired Users</button>
      <?php } ?>
  <?php // Retention cleanup: deletes vouchers that were never used and whose
        // batch comment is older than 30 days. Shown always, because it is not
        // tied to the current filter the way the two buttons above are. ?>
  <button class="btn bg-red" onclick="if(confirm('Remove UNUSED vouchers older than 30 days?\n\nOnly vouchers that have never been logged in with (uptime 0s) and whose comment date is older than 30 days are removed. Vouchers that were used are not touched.')){loadpage('./?remove-unused-hotspot-user=1&days=30&session=<?= $session; ?>');loader();}else{}" title="Remove unused vouchers older than 30 days">  <i class="fa fa-trash"></i> <?= $_unused_older ?></button>
  <script>
    /*
     * Yang dicetak adalah yang sedang terlihat di daftar: kalau sebuah Comment
     * dipilih, satu batch itu; kalau tidak tapi sebuah Profile dipilih, semua
     * voucher profil itu. Sebelumnya cuma Comment yang bisa, jadi untuk
     * mencetak satu batch orang harus tahu dulu bahwa Comment-nya harus
     * dipilih - dan tombolnya diam-diam tidak melakukan apa-apa kalau belum.
     */
    function printV(a,b){
    var comm = document.getElementById('comment').value;
    var prof = "<?= $prof; ?>";
    var url = "";
    if (comm !== ""){
      url = "./voucher/print.php?id="+encodeURIComponent(comm)+"&"+a+"="+b+"&session=<?= $session; ?>";
    } else if (prof !== "" && prof !== "all"){
      url = "./voucher/print.php?profile="+encodeURIComponent(prof)+"&"+a+"="+b+"&session=<?= $session; ?>";
    }
    if (url === ""){
      <?php if ($currency == in_array($currency, $cekindo['indo'])) { ?>
      alert('Silakan pilih salah satu Comment atau Profile terlebih dulu!');
      <?php
    } else { ?>
      alert('Please choose a Comment or a Profile first!');
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
if ($status != "" && $status != "all") {
  $pf_base .= "&status=" . urlencode($status);
} elseif ($exp == "1") {
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
  echo "</td>";
  echo "</tr>";
}
?>
  </tbody>
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

	
	
