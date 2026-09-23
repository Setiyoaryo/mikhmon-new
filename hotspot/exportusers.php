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
  $prof = isset($_GET['profile']) ? trim($_GET['profile']) : (isset($prof) ? $prof : '');
  $comm = isset($_GET['comment']) ? trim($_GET['comment']) : (isset($comm) ? $comm : '');

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
}
?>
<div class="row">
<div class="col-12">
<div class="card">
<div class="card-header">
    <h3><i class="fa fa-users"></i> Export Hotspot Users | <strong class="pointer" onclick="exportTableToCSV('export-user-hotspot-mikhmon-<?= date("Y-m-d"); ?>.<?php if ($_GET['export'] == "csv") {
                                                                                                                                                                  echo "csv";
                                                                                                                                                                } else {
                                                                                                                                                                  echo "txt";
                                                                                                                                                                } ?>')" title="Download User List"><i class="fa fa-download"></i> Download</strong>
    </h3>
    
</div>

<div class="card-body overflow">		   
<table id="export" class="text-nowrap <?php if ($_GET['export'] == "csv") {
                                        echo "table table-bordered";
                                      } ?>">
  <?php if ($_GET['export'] == "script") { ?>
  <tr>
    <td>/ip hotspot user</td>
  </tr>
<?php
if ($TotalReg == 0) {
  echo "<tr><td># No users found</td></tr>";
} else {
for ($i = 0; $i < $TotalReg; $i++) {
  $userdetails = $getuser[$i];
  $uid = isset($userdetails['.id']) ? $userdetails['.id'] : '';
  $userver = isset($userdetails['server']) ? $userdetails['server'] : '';
  $uname = isset($userdetails['name']) ? $userdetails['name'] : '';
  $upass = isset($userdetails['password']) ? $userdetails['password'] : '';
  $uprofile = isset($userdetails['profile']) ? $userdetails['profile'] : '';
  $uuptime = isset($userdetails['uptime']) ? formatDTM($userdetails['uptime']) : '';
  $ubyteso = isset($userdetails['bytes-out']) ? formatBytes($userdetails['bytes-out'], 2) : '';
  if ($ubyteso == 0) {
    $ubyteso = "";
  } else {
    $ubyteso = $ubyteso;
  }
  $ucomment = isset($userdetails['comment']) ? $userdetails['comment'] : '';
  $udisabled = isset($userdetails['disabled']) ? $userdetails['disabled'] : '';
  $utimelimit = isset($userdetails['limit-uptime']) ? $userdetails['limit-uptime'] : '';
  $udatalimit = isset($userdetails['limit-bytes-total']) ? $userdetails['limit-bytes-total'] : '';

  if ($utimelimit == "") {
    $timelimit = "";
  } else {
    $timelimit = 'limit-uptime="' . $utimelimit . '"';
  }
  if ($udatalimit == "") {
    $datalimit = "";
  } else {
    $datalimit = 'limit-bytes-total="' . $udatalimit . '"';
  }
  if ($ucomment == "") {
    $comment = "";
  } else {
    $comment = 'comment="' . $ucomment . '"';
  }

  echo '
  <tr>
    <td>add name="' . $uname . '" password="' . $upass . '" profile="' . $uprofile . '" ' . $comment . ' ' . $timelimit . ' ' . $datalimit . '</td>
  </tr>
	';
}
}
} else if ($_GET['export'] == "csv") { ?>
  <tr>
    <th>Username</th>
    <th>Password</th>
    <th>Profile</th>
    <th>Time Limit</th>
    <th>Data Limit</th>
    <th>Comment</th>
  </tr>
  <?php
  if ($TotalReg == 0) {
    echo "<tr><td colspan='6' class='text-center text-muted'>No users found</td></tr>";
  } else {
  for ($i = 0; $i < $TotalReg; $i++) {
    $userdetails = $getuser[$i];
    $uid = isset($userdetails['.id']) ? $userdetails['.id'] : '';
    $userver = isset($userdetails['server']) ? $userdetails['server'] : '';
    $uname = isset($userdetails['name']) ? $userdetails['name'] : '';
    $upass = isset($userdetails['password']) ? $userdetails['password'] : '';
    $uprofile = isset($userdetails['profile']) ? $userdetails['profile'] : '';
    $uuptime = isset($userdetails['uptime']) ? formatDTM($userdetails['uptime']) : '';
    $ubyteso = isset($userdetails['bytes-out']) ? formatBytes($userdetails['bytes-out'], 2) : '';
    if ($ubyteso == 0) {
      $ubyteso = "";
    } else {
      $ubyteso = $ubyteso;
    }
    $ucomment = isset($userdetails['comment']) ? $userdetails['comment'] : '';
    $udisabled = isset($userdetails['disabled']) ? $userdetails['disabled'] : '';
    $utimelimit = isset($userdetails['limit-uptime']) ? $userdetails['limit-uptime'] : '';
    $udatalimit = isset($userdetails['limit-bytes-total']) ? $userdetails['limit-bytes-total'] : '';

    echo '
  <tr>
    <td>' . $uname . '</td>
    <td>' . $upass . '</td>
    <td>' . $uprofile . '</td>
    <td>' . $utimelimit . '</td>
    <td>' . $udatalimit . '</td>
    <td>' . $ucomment . '</td>
  </tr>
  ';
  }
  }
}
?>

</table>
</div>
</div>
</div>
</div>
<script>
      function downloadCSV(csv, filename) {
        var csvFile;
        var downloadLink;
        // CSV file
        csvFile = new Blob([csv], {type: "text/csv"});
        // Download link
        downloadLink = document.createElement("a");
        // File name
        downloadLink.download = filename;
        // Create a link to the file
        downloadLink.href = window.URL.createObjectURL(csvFile);
        // Hide download link
        downloadLink.style.display = "none";
        // Add the link to DOM
        document.body.appendChild(downloadLink);
        // Click download link
        downloadLink.click();
        }
        
        function exportTableToCSV(filename) {
          var csv = [];
          var rows = document.querySelectorAll("#export tr");
          
         for (var i = 0; i < rows.length; i++) {
            var row = [], cols = rows[i].querySelectorAll("td, th");
         for (var j = 0; j < cols.length; j++)
            row.push(cols[j].innerText);
        csv.push(row.join(","));
        }
        // Download CSV file
        downloadCSV(csv.join("\n"), filename);
        }
</script>
	
	
