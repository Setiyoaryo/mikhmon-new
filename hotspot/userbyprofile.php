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
if (!isset($_SESSION["mikhmon"])) {
  header("Location:../admin.php?id=login");
} else {
// array color
  $color = array('1' => 'bg-blue', 'bg-indigo', 'bg-purple', 'bg-pink', 'bg-red', 'bg-yellow', 'bg-green', 'bg-teal', 'bg-cyan', 'bg-grey', 'bg-light-blue');

  include_once(dirname(__FILE__) . '/../include/voucherharga.php');
  include_once(dirname(__FILE__) . '/../include/hscache.php');

  $batch_summary = mikhmon_get_batches_summary($API, $session);
  $allusers = mikhmon_hscache_hotspot_users($API, $session);
  $counts = array();
  $countall = 0;
  if (is_array($allusers)) {
    foreach ($allusers as $u) {
      $p = isset($u['profile']) ? $u['profile'] : "";
      if (!isset($counts[$p])) {
        $counts[$p] = 0;
      }
      $counts[$p]++;
      $countall++;
    }
  }

  ?>
<div class="row">
<div class="col-12">
<div class="card">
<div class="card-header">
	<h3><i class=" fa fa-users"></i> <?= $_vouchers ?> &nbsp;&nbsp; | &nbsp;&nbsp;<i onclick="location.reload();" class="fa fa-refresh pointer" title="Reload data"></i></h3>
</div>
<div class="card-body">
<div class="overflow" style="max-height: 80vh">	
<div class="row">	
      <div class="col-4">
        <div class="box bmh-75 box-bordered <?= $color[rand(1, 11)]; ?>">
          <div class="box-group">
            <div class="box-group-icon">
              <a title='Open User by profile <?= $pname; ?>'  href='./?hotspot=users&profile=all&session=<?= $session; ?>'>
              <i class="fa fa-ticket"></i></a>
            </div>
              <div class="box-group-area">
                <h3 >Profile : all<br>
                <?php
                $countuser = $countall;
                if ($countuser < 2) {
                  echo $countuser . " Item";
                } elseif ($countuser > 1) {
                  echo $countuser . " Items";
                }
                ?></h3>

              <a title="Open User by profile all" href="./?hotspot=users&profile=all&session=<?= $session; ?>"><i class="fa fa-external-link"></i> <?= $_open ?></a>&nbsp;
              <a title="Generate User by profile <?= $pname; ?>" href="./?hotspot-user=generate&session=<?= $session; ?>"><i class="fa fa-users"></i> <?= $_generate ?></a>&nbsp;
              </div>
            </div>
            
          </div>
        </div>
<?php
// get user profile
$getprofile = $API->comm("/ip/hotspot/user/profile/print");
$TotalReg = count($getprofile);
for ($i = 0; $i < $TotalReg; $i++) {
  $profiledetalis = $getprofile[$i];
  $pname = $profiledetalis['name'];
  ?>
	     <div class="col-4">
        <div class="box bmh-75 box-bordered <?= $color[rand(1, 11)]; ?>">
          <div class="box-group">
            <div class="box-group-icon">
              <a title='Open User by profile <?= $pname; ?>'  href='./?hotspot=users&profile=<?= $pname; ?>&session=<?= $session; ?>'>
            	<i class="fa fa-ticket"></i></a>
            </div>
              <div class="box-group-area">
                <h3 >Profile : <?= $pname; ?><br>
                <?php	$countuser = isset($counts[$pname]) ? $counts[$pname] : 0;
                if ($countuser < 2) {
                  echo $countuser . " Item";
                } elseif ($countuser > 1) {
                  echo $countuser . " Items";
                }
                ?></h3>

              <a title="Open User by profile <?= $pname; ?>" href="./?hotspot=users&profile=<?= $pname; ?>&session=<?= $session; ?>"><i class="fa fa-external-link"></i> <?= $_open ?></a>&nbsp;
              <a title="Generate User by profile <?= $pname; ?>" href="./?hotspot-user=generate&genprof=<?= $pname; ?>&session=<?= $session; ?>"><i class="fa fa-users"></i> <?= $_generate ?></a>&nbsp;
              </div>
            </div>
            
          </div>
        </div>
        <?php 
      }
    ?>
      </div>
    </div>
</div>
</div>
</div>
</div>
<?php
// Batch summary section
$batch_count = count($batch_summary);
?>
<div class="row mr-t-10">
<div class="col-12">
<div class="card">
<div class="card-header">
  <h3><i class="fa fa-th-list"></i> <?= $_batch_summary; ?> (<?= $batch_count; ?>)
    <span style="font-size: 14px; float: right;">
      <a href="./?hotspot-user=generate&session=<?= $session; ?>" title="Generate User"><i class="fa fa-users"></i> <?= $_generate; ?></a>
    </span>
  </h3>
</div>
<div class="card-body">
  <div class="row pd-b-5">
    <div class="col-6">
      <input id="filterBatchTable" type="text" style="padding:5.8px;" class="group-item radius-3" placeholder="<?= $_search; ?>...">
    </div>
    <div class="col-6 text-right" style="padding-top:6px;">
      <span class="text-muted"><i class="fa fa-info-circle"></i> Batch voucher terdaftar & router</span>
    </div>
  </div>
  <div class="overflow mr-t-5 box-bordered" style="max-height: 60vh">
    <table id="batchTable" class="table table-bordered table-hover text-nowrap">
      <thead>
        <tr>
          <th style="width:40px;" class="text-center align-middle">#</th>
          <th class="align-middle"><?= $_batch_code; ?></th>
          <th class="align-middle"><?= $_profile; ?></th>
          <th class="align-middle"><?= $_date; ?></th>
          <th class="text-center align-middle"><?= $_total_generated; ?></th>
          <th class="text-center align-middle"><?= $_ready_stock; ?></th>
          <th class="text-center align-middle"><?= $_in_use; ?></th>
          <th class="text-right align-middle"><?= $_price; ?></th>
          <th class="text-center align-middle">Print</th>
          <th class="text-center align-middle"><?= $_action; ?></th>
        </tr>
      </thead>
      <tbody>
        <?php
        if ($batch_count === 0) {
          echo "<tr><td colspan='10' class='text-center pd-10'>Tidak ada batch voucher yang ditemukan</td></tr>";
        } else {
          $no = 1;
          foreach ($batch_summary as $b) {
            $b_code = $b['code'];
            $b_prof = $b['profile'];
            $b_date = ($b['time'] > 0) ? date("d/m/Y H:i", $b['time']) : "-";
            $b_total = (int)$b['total'];
            $b_ready = (int)$b['ready'];
            $b_used = (int)$b['used'];

            $b_price_disp = "-";
            if ($b['sprice'] !== '' && $b['sprice'] !== '0') {
              $b_price_disp = $currency . " " . number_format((float)$b['sprice'], 0, ",", ".");
            } elseif ($b['price'] !== '' && $b['price'] !== '0') {
              $b_price_disp = $currency . " " . number_format((float)$b['price'], 0, ",", ".");
            }

            $print_def = "./voucher/print.php?id=" . urlencode($b_code) . "&qr=no&session=" . $session;
            $print_qr = "./voucher/print.php?id=" . urlencode($b_code) . "&qr=yes&session=" . $session;
            $print_sm = "./voucher/print.php?id=" . urlencode($b_code) . "&small=yes&session=" . $session;
            $view_url = "./?hotspot=users&comment=" . urlencode($b_code) . "&session=" . $session;

            echo "<tr>";
            echo "<td class='text-center align-middle'>" . $no++ . "</td>";
            echo "<td class='align-middle'><a href='" . $view_url . "' title='Filter user batch'><b>" . htmlspecialchars($b_code, ENT_QUOTES) . "</b></a></td>";
            echo "<td class='align-middle'>" . htmlspecialchars($b_prof, ENT_QUOTES) . "</td>";
            echo "<td class='align-middle'>" . $b_date . "</td>";
            echo "<td class='text-center align-middle'><b>" . number_format($b_total, 0, ",", ".") . "</b></td>";
            echo "<td class='text-center align-middle'><span class='text-success' style='font-weight:bold;'>" . number_format($b_ready, 0, ",", ".") . "</span></td>";
            echo "<td class='text-center align-middle'><span class='text-danger' style='font-weight:bold;'>" . number_format($b_used, 0, ",", ".") . "</span></td>";
            echo "<td class='text-right align-middle'>" . $b_price_disp . "</td>";
            echo "<td class='text-center align-middle'>";
            if ($b_ready === 0) {
              echo "<span class='btn bg-secondary pd-2p5 mr-2 text-muted' style='opacity:0.4; cursor:not-allowed;' title='Tidak ada stok ready'><i class='fa fa-print'></i></span>";
              echo "<span class='btn bg-secondary pd-2p5 mr-2 text-muted' style='opacity:0.4; cursor:not-allowed;' title='Tidak ada stok ready'><i class='fa fa-qrcode'></i></span>";
              echo "<span class='btn bg-secondary pd-2p5 text-muted' style='opacity:0.4; cursor:not-allowed;' title='Tidak ada stok ready'><i class='fa fa-print'></i></span>";
            } else {
              echo "<a class='btn bg-primary pd-2p5 mr-2' target='_blank' href='" . $print_def . "' title='Print Standard'><i class='fa fa-print'></i></a>";
              echo "<a class='btn bg-danger pd-2p5 mr-2' target='_blank' href='" . $print_qr . "' title='Print QR'><i class='fa fa-qrcode'></i></a>";
              echo "<a class='btn bg-info pd-2p5' target='_blank' href='" . $print_sm . "' title='Print Small'><i class='fa fa-print'></i></a>";
            }
            echo "</td>";
            echo "<td class='text-center align-middle'>";
            echo "<a class='btn bg-secondary pd-2p5 mr-2' href='" . $view_url . "' title='Lihat voucher'><i class='fa fa-search'></i></a>";
            echo "<button class='btn bg-red pd-2p5' onclick=\"if(confirm('Hapus seluruh voucher dalam batch (" . htmlspecialchars($b_code, ENT_QUOTES) . ")?')){loadpage('./?remove-hotspot-user-by-comment=" . urlencode($b_code) . "&session=" . $session . "');loader();}\" title='Hapus batch'><i class='fa fa-trash'></i></button>";
            echo "</td>";
            echo "</tr>";
          }
        }
        ?>
      </tbody>
    </table>
  </div>
</div>
</div>
</div>
</div>
<script>
$(document).ready(function(){
  $("#filterBatchTable").on("keyup", function() {
    var value = $(this).val().toLowerCase();
    $("#batchTable tbody tr").filter(function() {
      $(this).toggle($(this).text().toLowerCase().indexOf(value) > -1)
    });
  });
});
</script>
<?php 
} ?>