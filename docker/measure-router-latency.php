<?php
/*
 * Measure the cost of one RouterOS round trip through the same code path the
 * UI uses, so a slow menu can be attributed to the network instead of guessed.
 *
 * Run it inside the php container:
 *
 *   docker exec mikhmon-php php /var/www/docker/measure-router-latency.php <session>
 *
 * <session> is the Session Name from Session Settings (for example HOTSPOT).
 * Nothing is written anywhere; it only reads.
 */

require '/var/www/lib/routeros_api.class.php';

$session = isset($argv[1]) ? $argv[1] : '';
if ($session === '') {
    fwrite(STDERR, "usage: php measure-router-latency.php <session-name>\n");
    exit(1);
}

// readcfg.php expects these to exist.
$_SERVER['REQUEST_URI'] = '/measure';
$_GET['session'] = $session;

$data = array();
include '/var/www/include/config.php';
include '/var/www/include/readcfg.php';

if (!isset($data[$session])) {
    fwrite(STDERR, "session '$session' is not in include/config.php\n");
    exit(1);
}

$ip   = $iphost;
$user = $userhost;
$pass = decrypt($passwdhost);

echo "session   : $session\n";
echo "router    : $ip\n";
echo "hotspot   : $hotspotname\n";
echo "\n";

// 1. Dial and login from cold (what a page pays when the pool is empty).
$API = new RouterosAPI();
$API->debug = false;
$API->timeout = 10;
$t0 = microtime(true);
$ok = $API->connect($ip, $user, $pass);
$t1 = microtime(true);
if (!$ok) {
    fwrite(STDERR, "could not connect: " . $API->error_str . "\n");
    exit(1);
}
printf("connect (dial + login)      : %6.1f ms\n", ($t1 - $t0) * 1000);

// 2. Warm round trips, which is what every later call costs.
$n = 10;
$t2 = microtime(true);
for ($i = 0; $i < $n; $i++) {
    $API->comm('/system/clock/print');
}
$t3 = microtime(true);
$perCall = ($t3 - $t2) / $n * 1000;
printf("%d warm round trips          : %6.1f ms total, %.1f ms each\n", $n, ($t3 - $t2) * 1000, $perCall);

// 3. Turn that into what a page costs, using the measured round trip counts.
echo "\npredicted page time (server side, browser latency not included):\n";
foreach (array('Dashboard' => 2, 'Hosts / Cookies / IP binding / DHCP' => 1,
               'User List / Profile List' => 3, 'Hotspot Active' => 3) as $page => $trips) {
    printf("  %-36s %2d round trips -> %6.0f ms\n", $page, $trips, $trips * $perCall);
}
echo "\nIf those numbers are large, the cost is the link to the router, not the app.\n";
