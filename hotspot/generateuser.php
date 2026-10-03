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

// Voucher batches of 5000 are supported; the Go backend does the work in
// parallel, but leave room for a slow router on top of the HTTP round trip.
ini_set('max_execution_time', 600);

if (!isset($_SESSION["mikhmon"])) {
	header("Location:../admin.php?id=login");
} else {
// time zone
date_default_timezone_set(empty($_SESSION['timezone']) ? 'Asia/Jakarta' : $_SESSION['timezone']);

	$genprof = $_GET['genprof'];
	if ($genprof != "") {
		$getprofile = $API->comm("/ip/hotspot/user/profile/print", array(
			"?name" => "$genprof",
		));
		$ponlogin = $getprofile[0]['on-login'];
		$getprice = explode(",", $ponlogin)[2];
		if ($getprice == "0") {
			$getprice = "";
		} else {
			$getprice = $getprice;
		}

		$getvalid = explode(",", $ponlogin)[3];

		$getlocku = explode(",", $ponlogin)[6];
		if ($getlocku == "") {
			$getprice = "Disable";
		} else {
			$getlocku = $getlocku;
		}

		if ($currency == in_array($currency, $cekindo['indo'])) {
			$getprice = $currency . " " . number_format((float)$getprice, 0, ",", ".");
		} else {
			$getprice = $currency . " " . number_format((float)$getprice);
		}
		$ValidPrice = "<b>Validity : " . $getvalid . " | Price : " . $getprice . " | Lock User : " . $getlocku . "</b>";
	} else {
	}

	$srvlist = $API->comm("/ip/hotspot/print");

	if (isset($_POST['qty'])) {
		
		$qty = ($_POST['qty']);
		$server = ($_POST['server']);
		$user = ($_POST['user']);
		$userl = ($_POST['userl']);
		$prefix = ($_POST['prefix']);
		$char = ($_POST['char']);
		$profile = ($_POST['profile']);
		$timelimit = trim((string) $_POST['timelimit']);
		$datalimit = ($_POST['datalimit']);
		$adcomment = ($_POST['adcomment']);
		$mbgb = ($_POST['mbgb']);
		// Kosong dan "0" sama-sama berarti tanpa batas uptime. Dibiarkan kosong
		// supaya atribut limit-uptime tidak dikirim ke RouterOS sama sekali
		// (nilai kosong ditolak; "0" berisiko terbaca sebagai batas 0 detik).
		if ($timelimit == "0") {
			$timelimit = "";
		}
		if ($datalimit == "") {
			$datalimit = "0";
		} else {
			$datalimit = $datalimit * $mbgb;
		}
		if ($adcomment == "") {
			$adcomment = "";
		} else {
			$adcomment = $adcomment;
		}
		$getprofile = $API->comm("/ip/hotspot/user/profile/print", array("?name" => "$profile"));
		$ponlogin = $getprofile[0]['on-login'];
		$getvalid = explode(",", $ponlogin)[3];
		$getprice = explode(",", $ponlogin)[2];
		$getsprice = explode(",", $ponlogin)[4];
		$getlock = explode(",", $ponlogin)[6];
		/*
		 * Harga jual yang diisi di sini berlaku untuk batch ini saja. Kalau
		 * dikosongkan, harganya tetap dari profil seperti sebelumnya - jadi
		 * cara lama tidak berubah. Tanpa kolom ini, satu-satunya cara
		 * mengubah harga voucher adalah mengubah profilnya, dan itu ikut
		 * mengubah voucher lama kalau dicetak ulang.
		 */
		$vsprice = isset($_POST['vsprice']) ? preg_replace('/[^0-9]/', '', (string) $_POST['vsprice']) : '';
		if ($vsprice !== '') {
			$getsprice = $vsprice;
		}
		$_SESSION['ubp'] = $profile;
		// Kode batch: angka acak 5 digit + tanggal. Dulu hanya 3 digit (900
		// kemungkinan), sehingga dua batch di hari yang sama bisa kebetulan
		// berbagi kode - dan saat cetak, dua batch itu tercampur. 5 digit
		// memperkecil peluang tabrakan 100x.
		$commt = $user . "-" . rand(10000, 99999) . "-" . date("m.d.y") . "-" . $adcomment;
		include_once(dirname(__FILE__) . '/../include/voucherharga.php');
		mikhmon_voucher_harga_put($commt, array(
			'price'   => $getprice,
			'sprice'  => $getsprice,
			'profile' => $profile,
			'qty'     => $qty,
		));
		$gentemp = $commt . "|~" . $profile . "~" . $getvalid . "~" . $getprice . "!".$getsprice."~" . $timelimit . "~" . $datalimit . "~" . $getlock;
		$gen = '<?php $genu="'.encrypt($gentemp).'";?>';
		$tempDir = dirname(__FILE__) . '/../data/voucher';
		if (!is_dir($tempDir)) {
			@mkdir($tempDir, 0777, true);
		}
		$temp = is_dir($tempDir) ? $tempDir . '/temp.php' : './voucher/temp.php';
		$handle = @fopen($temp, 'w');
		if ($handle) {
			fwrite($handle, $gen);
			fclose($handle);
		}

		// Credentials, uniqueness, and router writes are all owned by Go.
		$bulkresult = mikhmon_generate_hotspot_users($API, array(
			'qty' => (int) $qty,
			'server' => $server,
			'mode' => $user,
			'userl' => (int) $userl,
			'prefix' => $prefix,
			'char' => $char,
			'profile' => $profile,
			'timelimit' => $timelimit,
			'datalimit' => (int) $datalimit,
			'comment' => $commt,
		));
		$u = array(1 => isset($bulkresult['first_user']) ? $bulkresult['first_user'] : '');


		/* Simpan hasil supaya halaman berikutnya bisa menampilkan berapa yang
		 * benar-benar jadi dan berapa yang gagal - dulu tidak ada umpan balik
		 * sama sekali, jadi voucher yang gagal terbuat tidak terlihat. */
		$hasil = is_array($bulkresult) ? $bulkresult : array();
		$_SESSION['mikhmon_generate_hasil'] = array(
			'comment' => $commt,
			'profile' => $profile,
			'qty'     => $qty,
			'added'   => isset($hasil['added']) ? (int) $hasil['added'] : 0,
			'failed'  => isset($hasil['failed']) ? (int) $hasil['failed'] : 0,
			'errors'  => (isset($hasil['errors']) && is_array($hasil['errors'])) ? array_slice($hasil['errors'], 0, 10) : array(),
		);

		if ($qty < 2 && (int) $_SESSION['mikhmon_generate_hasil']['failed'] === 0) {
			echo "<script>window.location='./?hotspot-user=" . $u[1] . "&session=" . $session . "'</script>";
		} else {
			echo "<script>window.location='./?hotspot=users&comment=" . urlencode($commt)
				. "&profile=" . urlencode($profile) . "&session=" . $session . "'</script>";
		}
	}

	$getprofile = $API->comm("/ip/hotspot/user/profile/print");
	$dataTemp = dirname(__FILE__) . '/../data/voucher/temp.php';
	if (is_file($dataTemp)) {
		include_once($dataTemp);
	} else {
		include_once('./voucher/temp.php');
	}
	$genuser = explode("-", decrypt($genu));
	$genuser1 = explode("~", decrypt($genu));
	$umode = $genuser[0];
	$ucode = $genuser[1];
	$udate = $genuser[2];
	$uprofile = $genuser1[1];
	$uvalid = $genuser1[2];
	$ucommt = $genuser[3];
	if ($uvalid == "") {
		$uvalid = "-";
	} else {
		$uvalid = $uvalid;
	}
	$uprice = explode("!",$genuser1[3])[0];
	if ($uprice == "0") {
		$uprice = "-";
	} else {
		$uprice = $uprice;
	}
	$suprice = explode("!",$genuser1[3])[1];
	if ($suprice == "0") {
		$suprice = "-";
	} else {
		$suprice = $suprice;
	}
	$utlimit = $genuser1[4];
	if ($utlimit == "0") {
		$utlimit = "-";
	} else {
		$utlimit = $utlimit;
	}
	$udlimit = $genuser1[5];
	if ($udlimit == "0") {
		$udlimit = "-";
	} else {
		$udlimit = formatBytes($udlimit, 2);
	}
	$ulock = $genuser1[6];
	//$urlprint = "$umode-$ucode-$udate-$ucommt";
	$urlprint = explode("|", decrypt($genu))[0];
	if ($currency == in_array($currency, $cekindo['indo'])) {
		$uprice = $currency . " " . number_format((float)$uprice, 0, ",", ".");
		$suprice = $currency . " " . number_format((float)$suprice, 0, ",", ".");
	} else {
		$uprice = $currency . " " . number_format((float)$uprice);
		$suprice = $currency . " " . number_format((float)$suprice);

	}

}
?>
<?php
/* Umpan balik setelah generate: berapa yang jadi, berapa yang gagal, dan
 * alasan kegagalannya. Ditampilkan sekali lalu dibuang.
 *
 * Hanya pada GET: saat POST, halaman ini juga ikut render setelah menulis
 * script redirect - kalau pesan dibuang di situ, halaman tujuan tidak akan
 * pernah melihatnya. */
if ($_SERVER['REQUEST_METHOD'] !== 'POST' && isset($_SESSION['mikhmon_generate_hasil']) && is_array($_SESSION['mikhmon_generate_hasil'])) {
  $gh = $_SESSION['mikhmon_generate_hasil'];
  unset($_SESSION['mikhmon_generate_hasil']);
  $gh_added = isset($gh['added']) ? (int) $gh['added'] : 0;
  $gh_failed = isset($gh['failed']) ? (int) $gh['failed'] : 0;
  $gh_errors = (isset($gh['errors']) && is_array($gh['errors'])) ? $gh['errors'] : array();
  ?>
<div class="row">
  <div class="col-12">
    <div style="margin:8px 0; padding:10px 14px; border-radius:4px; border-left:4px solid <?= $gh_failed > 0 ? '#e74c3c' : '#27ae60'; ?>; background:<?= $gh_failed > 0 ? '#fdecea' : '#eafaf1'; ?>;">
      <b><?= number_format($gh_added, 0, ",", "."); ?> voucher jadi</b>
      untuk profil <b><?= htmlspecialchars(isset($gh['profile']) ? $gh['profile'] : '', ENT_QUOTES); ?></b>
      &middot; batch <span style="font-family:monospace;"><?= htmlspecialchars(isset($gh['comment']) ? $gh['comment'] : '', ENT_QUOTES); ?></span>
      <?php if ($gh_failed > 0) { ?>
        &middot; <b style="color:#c0392b;"><?= (int) $gh_failed; ?> GAGAL</b>
      <?php } ?>
      <?php if (!empty($gh_errors)) { ?>
        <div style="margin-top:6px; font-size:12px; color:#c0392b;">
          <?php foreach ($gh_errors as $ge) { echo htmlspecialchars($ge, ENT_QUOTES) . "<br>"; } ?>
        </div>
      <?php } ?>
    </div>
  </div>
</div>
  <?php
}
?>
<div class="row">
	
<div class="col-8">
<div class="card box-bordered">
	<div class="card-header">
	<h3><i class="fa fa-user-plus"></i> <?= $_generate_user ?> <small id="loader" style="display: none;" ><i><i class='fa fa-circle-o-notch fa-spin'></i> <?= $_processing ?> </i></small></h3> 
	</div>
	<div class="card-body">
<form autocomplete="off" method="post" action="">
	<div>
		<?php if ($_SESSION['ubp'] != "") {
		echo "    <a class='btn bg-warning' href='./?hotspot=users&profile=" . $_SESSION['ubp'] . "&session=" . $session . "'> <i class='fa fa-close'></i> ".$_close."</a>";
	} elseif ($_SESSION['vcr'] = "active") {
		echo "    <a class='btn bg-warning' href='./?hotspot=users-by-profile&session=" . $session . "'> <i class='fa fa-close'></i> ".$_close."</a>";
	} else {
		echo "    <a class='btn bg-warning' href='./?hotspot=users&profile=all&session=" . $session . "'> <i class='fa fa-close'></i> ".$_close."</a>";
	}

	?>
	<a class="btn bg-pink" title="Open User List by Profile 
<?php if ($_SESSION['ubp'] == "") {
	echo "all";
} else {
	echo $uprofile;
} ?>" href="./?hotspot=users&profile=
<?php if ($_SESSION['ubp'] == "") {
	echo "all";
} else {
	echo $uprofile;
} ?>&session=<?= $session; ?>"> <i class="fa fa-users"></i> <?= $_user_list ?></a>
    <button type="submit" name="save" onclick="loader()" class="btn bg-primary" title="Generate User"> <i class="fa fa-save"></i> <?= $_generate ?></button>
    <a class="btn bg-secondary" title="Print Default" href="./voucher/print.php?id=<?= $urlprint; ?>&qr=no&session=<?= $session; ?>" target="_blank"> <i class="fa fa-print"></i> <?= $_print ?></a>
    <a class="btn bg-danger" title="Print QR" href="./voucher/print.php?id=<?= $urlprint; ?>&qr=yes&session=<?= $session; ?>" target="_blank"> <i class="fa fa-qrcode"></i> <?= $_print_qr ?></a>
    <a class="btn bg-info" title="Print Small" href="./voucher/print.php?id=<?= $urlprint; ?>&small=yes&session=<?= $session; ?>" target="_blank"> <i class="fa fa-print"></i> <?= $_print_small ?></a>
</div>
<table class="table">
  <tr>
    <td class="align-middle"><?= $_qty ?></td><td><div><input class="form-control " type="number" name="qty" min="1" max="5000" value="1" required="1"></div></td>
  </tr>
  <tr>
    <td class="align-middle">Server</td>
    <td>
		<select class="form-control " name="server" required="1">
			<option>all</option>
				<?php $TotalReg = count($srvlist);
			for ($i = 0; $i < $TotalReg; $i++) {
				echo "<option>" . $srvlist[$i]['name'] . "</option>";
			}
			?>
		</select>
	</td>
	</tr>
	<tr>
    <td class="align-middle"><?= $_user_mode ?></td><td>
			<select class="form-control " onchange="defUserl();" id="user" name="user" required="1">
				<option value="up"><?= $_user_pass ?></option>
				<option value="vc"><?= $_user_user ?></option>
			</select>
		</td>
	</tr>
  <tr>
    <td class="align-middle"><?= $_user_length ?></td><td>
      <select class="form-control " id="userl" name="userl" required="1">
        <option>4</option>
				<option>3</option>
				<option>4</option>
				<option>5</option>
				<option>6</option>
				<option>7</option>
				<option>8</option>
			</select>
    </td>
  </tr>
  <tr>
    <td class="align-middle"><?= $_prefix ?></td><td><input class="form-control " type="text" size="6" maxlength="6" autocomplete="off" name="prefix" value=""></td>
  </tr>
  <tr>
    <td class="align-middle"><?= $_character ?></td><td>
      <select class="form-control " name="char" required="1">
				<option id="lower" style="display:block;" value="lower"><?= $_random ?> abcd</option>
				<option id="upper" style="display:block;" value="upper"><?= $_random ?> ABCD</option>
				<option id="upplow" style="display:block;" value="upplow"><?= $_random ?> aBcD</option>
				<option id="lower1" style="display:none;" value="lower"><?= $_random ?> abcd2345</option>
				<option id="upper1" style="display:none;" value="upper"><?= $_random ?> ABCD2345</option>
				<option id="upplow1" style="display:none;" value="upplow"><?= $_random ?> aBcD2345</option>
				<option id="mix" style="display:block;" value="mix"><?= $_random ?> 5ab2c34d</option>
				<option id="mix1" style="display:block;" value="mix1"><?= $_random ?> 5AB2C34D</option>
				<option id="mix2" style="display:block;" value="mix2"><?= $_random ?> 5aB2c34D</option>
				<option id="num" style="display:none;" value="num"><?= $_random ?> 1234</option>
			</select>
    </td>
  </tr>
  <tr>
    <td class="align-middle"><?= $_profile ?></td><td>
			<select class="form-control " onchange="GetVP();" id="uprof" name="profile" required="1">
				<?php if ($genprof != "") {
				echo "<option>" . $genprof . "</option>";
			} else {
			}
			$TotalReg = count($getprofile);
			for ($i = 0; $i < $TotalReg; $i++) {
				echo "<option>" . $getprofile[$i]['name'] . "</option>";
			}
			?>
			</select>
		</td>
	</tr>
	<tr>
    <td class="align-middle"><?= $_time_limit ?></td>
    <td>
      <input class="form-control" type="text" size="4" autocomplete="off" name="timelimit" value="" placeholder="e.g. 1h, 30m" title="<?= isset($_time_limit_help) ? $_time_limit_help : '' ?>">
      <small style="color:#777;"><i class="fa fa-info-circle"></i> <?= isset($_time_limit_help) ? $_time_limit_help : '' ?></small>
    </td>
  </tr>
	<tr>
    <td class="align-middle"><?= $_data_limit ?></td><td>
      <div class="input-group">
      	<div class="input-group-10 col-box-9">
        	<input class="group-item group-item-l" type="number" min="0" max="9999" name="datalimit" value="<?= $udatalimit; ?>">
    	</div>
          <div class="input-group-2 col-box-3">
              <select style="padding:4.2px;" class="group-item group-item-r" name="mbgb" required="1">
				        <option value=1048576>MB</option>
				        <option value=1073741824>GB</option>
			        </select>
          </div>
      </div>
    </td>
  </tr>
	<tr>
    <td class="align-middle"><?= $_comment ?></td><td><input class="form-control " type="text" title="No special characters" id="comment" autocomplete="off" name="adcomment" value=""></td>
  </tr>
	<tr>
    <td class="align-middle"><?= $_selling_price ?></td><td>
      <div class="input-group">
        <div class="input-group-10 col-box-9">
          <input class="group-item group-item-l" type="number" min="0" autocomplete="off" name="vsprice" value="" placeholder="<?= $_profile ?>" title="Kosongkan untuk memakai harga dari profil">
        </div>
        <div class="input-group-2 col-box-3">
          <div class="group-item group-item-r pd-2p5 text-center" title="Kosongkan untuk memakai harga dari profil"><i class="fa fa-info-circle"></i></div>
        </div>
      </div>
    </td>
  </tr>
   <tr >
    <td  colspan="4" class="align-middle w-12"  id="GetValidPrice">
    	<?php if ($genprof != "") {
					echo $ValidPrice;
				} ?>
    </td>
  </tr>
</table>
</form>
</div>
</div>
</div>

<div class="col-4">
	<div class="card">
		<div class="card-header">
			<h3><i class="fa fa-ticket"></i> <?= $_last_generate ?></h3>
		</div>
		<div class="card-body">
<table class="table table-bordered">
  <tr>
  	<td><?= $_generate_code ?></td><td><?= $ucode ?></td>
  </tr>
  <tr>
  	<td><?= $_date ?></td><td><?= $udate ?></td>
  </tr>
  <tr>
  	<td><?= $_profile ?></td><td><?= $uprofile ?></td>
  </tr>
  <tr>
  	<td><?= $_validity ?></td><td><?= $uvalid ?></td>
  <tr>
  	<td><?= $_time_limit ?></td><td><?= $utlimit ?></td>
  </tr>
  <tr>
  	<td><?= $_data_limit ?></td><td><?= $udlimit ?></td>
  </tr>
  <tr>
  	<td><?= $_price ?></td><td><?= $uprice ?></td>
  </tr>
  <tr>
  	<td><?= $_selling_price ?></td><td><?= $suprice ?></td>
  </tr>
  <tr>
  	<td><?= $_lock_user ?></td><td><?= $ulock ?></td>
  </tr>
  <tr>
    <td colspan="2">
		<p style="padding:0px 5px;">
      <?= $_format_time_limit ?>
    </p>
    <p style="padding:0px 5px;">
      <?= $_details_add_user ?>
    </p>
    </td>
  </tr>
</table>
</div>
</div>
</div>
<script>
// get valid $ price
function GetVP(){
  var prof = document.getElementById('uprof').value;
  $("#GetValidPrice").load("./process/getvalidprice.php?name="+prof+"&session=<?= $session; ?> #getdata");
} 
</script>
</div>
