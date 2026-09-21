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
// One call finds the batch and deletes it. Doing it here would mean printing
// every matching user, shipping the whole list over HTTP and posting the ids
// straight back, which on a large table costs more than the removal itself.
//
// The query keeps the "never used" guard: a voucher that has been logged in
// with has a non zero uptime and is never selected.
$result = mikhmon_bulk_remove_by_query($API, array(
  "comment" => $removehotspotuserbycomment,
  "uptime"  => "00:00:00",
));

// Land back on the batch's own profile list, as before.
$_SESSION['ubp'] = isset($result['profile']) && $result['profile'] != "" ? $result['profile'] : "";
$_SESSION['ubc'] = "";
if ($_SESSION['ubp'] != "") {
  echo "<script>window.location='./?hotspot=users&profile=" . $_SESSION['ubp'] . "&session=" . $session . "'</script>";
} else {
  echo "<script>window.location='./?hotspot=users&profile=all&session=" . $session . "'</script>";
}

?>