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
// Both entry points do the same thing: delete the selected hotspot user(s)
// together with the per-user script and scheduler Mikhmon creates for them.
// The original code did three prints and three removes PER user, so deleting a
// few hundred vouchers meant a few hundred round trips. The lookups now happen
// once and every removal goes out as one parallel batch.

if ($removehotspotusers != "") {
  $uids = explode("~", $removehotspotusers);

  // One print to map .id -> name, instead of one print per selected user.
  $nameById = array();
  $allusers = $API->comm("/ip/hotspot/user/print");
  if (is_array($allusers)) {
    foreach ($allusers as $u) {
      if (isset($u['.id'])) {
        $nameById[$u['.id']] = isset($u['name']) ? $u['name'] : "";
      }
    }
  }
} else {
  $uids = array($removehotspotuser);

  $nameById = array();
  $getuname = $API->comm("/ip/hotspot/user/print", array(
    "?.id" => "$removehotspotuser",
  ));
  if (isset($getuname[0]['.id'])) {
    $nameById[$getuname[0]['.id']] = isset($getuname[0]['name']) ? $getuname[0]['name'] : "";
  }
}

// The script and scheduler Mikhmon creates for a user carry the user's name.
$names = array();
foreach ($uids as $uid) {
  if (isset($nameById[$uid]) && $nameById[$uid] !== "") {
    $names[$nameById[$uid]] = true;
  }
}

$scrIds = array();
$schIds = array();
if (!empty($names)) {
  $getscr = $API->comm("/system/script/print");
  if (is_array($getscr)) {
    foreach ($getscr as $s) {
      if (isset($s['name'], $s['.id']) && isset($names[$s['name']])) {
        $scrIds[] = $s['.id'];
      }
    }
  }

  $getsch = $API->comm("/system/scheduler/print");
  if (is_array($getsch)) {
    foreach ($getsch as $s) {
      if (isset($s['name'], $s['.id']) && isset($names[$s['name']])) {
        $schIds[] = $s['.id'];
      }
    }
  }
}

mikhmon_bulk_remove_ids($API, "/system/script/remove", $scrIds);
mikhmon_bulk_remove_ids($API, "/system/scheduler/remove", $schIds);
mikhmon_bulk_remove_hotspot_users($API, $uids);

if ($_SESSION['ubp'] != "") {
  echo "<script>window.location='./?hotspot=users&profile=" . $_SESSION['ubp'] . "&session=" . $session . "'</script>";
} elseif ($_SESSION['ubc'] != "") {
  echo "<script>window.location='./?hotspot=users&comment=" . $_SESSION['ubc'] . "&session=" . $session . "'</script>";
} else {
  echo "<script>window.location='./?hotspot=users&profile=all&session=" . $session . "'</script>";
}
?>