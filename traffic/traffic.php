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
error_reporting(0);
header('Content-Type: application/json; charset=utf-8');
header('Cache-Control: no-store');
if (!isset($_SESSION['mikhmon'])) {
  http_response_code(401);
  echo json_encode(array('ok' => false, 'error' => 'not authenticated'));
  exit;
}
$session = isset($_GET['session']) && is_string($_GET['session']) ? $_GET['session'] : '';
$interface = isset($_GET['iface']) && is_string($_GET['iface']) ? $_GET['iface'] : '';
if ($interface === '' || $interface === 'null' || $session === '') {
  http_response_code(400);
  echo json_encode(array('ok' => false, 'error' => 'missing session or interface'));
  exit;
}
include('../include/config.php');
include('../include/readcfg.php');
// Sampling must not block other requests from the same PHP session.
session_write_close();
include_once('../lib/routeros_api.class.php');
$API = new RouterosAPI();
$API->debug = false;
if (!$API->connect($iphost, $userhost, decrypt($passwdhost))) {
  http_response_code(502);
  echo json_encode(array('ok' => false, 'error' => 'router connection failed'));
  exit;
}
$sample = mikhmon_api_post('/v1/traffic', array(
  'session' => $API->session,
  'interface' => $interface,
  'timeout_ms' => max(1000, (int) $API->timeout * 1000),
), mikhmon_api_exec_timeout());
$API->disconnect();
if (!is_array($sample) || empty($sample['ok']) || !isset($sample['tx'], $sample['rx'])) {
  http_response_code(502);
  echo json_encode(array('ok' => false, 'error' => 'traffic sample unavailable'));
  exit;
}
echo json_encode(array(
  array('name' => 'Tx', 'data' => array((int) $sample['tx'])),
  array('name' => 'Rx', 'data' => array((int) $sample['rx'])),
));
