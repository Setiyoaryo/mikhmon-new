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

// Retention cleanup: delete vouchers that were never used and are older than
// the requested age, so dead inventory does not pile up in the hotspot user
// table. Two conditions have to hold, and both are conservative:
//
//  1. uptime is still 0s - the voucher has never been logged in with. A
//     voucher that was used and has since logged out keeps its uptime, so it
//     is never touched by this cleanup.
//  2. the comment carries a date older than $days. Mikhmon writes the
//     generation date into every batch comment, e.g.
//         vc-735-09.22.26-warung-oktober
//                     ^^^^^^^^  MM.DD.YY
//     so age can be derived without any extra bookkeeping. A comment without
//     that shape (a user added by hand, for example) is skipped - nothing is
//     deleted that cannot be dated.
//
// The expiry of vouchers that WERE used is a different path: the user profile
// rewrites their limit-uptime to 1s, and "Expired Users" removes them.

if (isset($_SESSION['timezone']) && $_SESSION['timezone'] != "") {
  // The date in the comment was written with this timezone, so compare in it.
  date_default_timezone_set($_SESSION['timezone']);
}

$retention = isset($_GET['days']) ? (int) $_GET['days'] : 30;
if ($retention < 1) {
  $retention = 30;
}

$cutoff = time() - ($retention * 86400);

$getuser = $API->comm("/ip/hotspot/user/print", array(
  "?uptime" => "00:00:00",
));

$uids = array();
$TotalReg = is_array($getuser) ? count($getuser) : 0;

for ($i = 0; $i < $TotalReg; $i++) {
  $comment = isset($getuser[$i]['comment']) ? $getuser[$i]['comment'] : "";

  // The date is the third dash separated field of the comment.
  $parts = explode("-", $comment);
  if (!isset($parts[2]) || !preg_match('/^([0-9]{2})\.([0-9]{2})\.([0-9]{2})$/', $parts[2], $m)) {
    continue;
  }

  $stamp = mktime(0, 0, 0, (int) $m[1], (int) $m[2], 2000 + (int) $m[3]);
  if ($stamp === false || $stamp > $cutoff) {
    continue;
  }

  if (isset($getuser[$i]['.id']) && $getuser[$i]['.id'] !== "") {
    $uids[] = $getuser[$i]['.id'];
  }
}

$_SESSION['ubp'] = "";
$_SESSION['ubc'] = "";

mikhmon_bulk_remove_hotspot_users($API, $uids);

echo "<script>window.location='./?hotspot=users&profile=all&session=" . $session . "'</script>";

?>
