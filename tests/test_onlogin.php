<?php
require __DIR__ . '/../include/onlogin.php';

$failures = array();
function check_onlogin($condition, $message) {
    global $failures;
    if (!$condition) { $failures[] = $message; }
}

/* ---------------------------------------------------------------- build */

$script = mikhmon_build_onlogin('rem', '1000', '24h', '2000', '', 'Disable', '')['onlogin'];
check_onlogin(strpos($script, ':local mdays (31,28,') !== false, 'build must use direct date arithmetic');
check_onlogin(strpos($script, '[/ip hotspot user find where name="$user"] comment') !== false, 'build must read the user comment before stamping');
check_onlogin(strpos($script, 'start-time=$time') === false, 'build must not rely on the temporary scheduler anymore');
check_onlogin(strpos($script, '/sys sch add') === false, 'build must not create a temporary scheduler');
check_onlogin(strpos($script, ':error') === false, 'build must never abort the hotspot login with a fatal error');
check_onlogin(strpos($script, ':local val "24h"') !== false, 'validity literal must be embedded');
check_onlogin(strpos($script, '$ps X"') !== false, 'remove mode must end the comment with the X flag');

$notice = mikhmon_build_onlogin('ntf', '1000', '4h', '2000', '', 'Disable', '')['onlogin'];
check_onlogin(strpos($notice, '$ps N"') !== false, 'notice mode must end the comment with the N flag');
check_onlogin(mikhmon_build_onlogin('ntf', '1000', '4h', '2000', '', 'Disable', '')['mode'] === 'set limit-uptime=1s', 'notice mode maps to limit-uptime');

// Blok dan kurung harus seimbang agar RouterOS bisa mengompilasi script.
$full = mikhmon_build_onlogin('remc', '2000', '4h', '2000',
    '; [:local mac $"mac-address"; /ip hotspot user set mac-address=$mac [find where name=$user]]',
    'Enable',
    '; /system script add name="rec" comment="mikhmon"')['onlogin'];
check_onlogin(substr_count($full, '{') === substr_count($full, '}'), 'on-login braces must balance');
check_onlogin(substr_count($full, '[') === substr_count($full, ']'), 'on-login brackets must balance');
check_onlogin(strpos($full, ':error') === false, 'on-login must never contain a fatal :error');

/* --------------------------------------------------------------- upgrade */

// Scheduler-generation format (the one currently deployed on routers).
$sched = ':put (",remc,1000,24h,2000,,Disable,,mikhmon,2312005,"); '
    . '{:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; :local ucode [:pic $comment 0 2]; '
    . ':if ($ucode = "vc" or $ucode = "up" or $comment = "") do={ :local date [ /system clock get date ]; :local time [ /system clock get time ]; '
    . ':local schname ("exp-" . $user); /sys sch add name=$schname disable=no start-date=$date start-time=$time interval="24h"; '
    . ':local firstexp ("$date $time"); :local exp [ /sys sch get [ /sys sch find where name=$schname ] next-run]; /sys sch remove [find where name=$schname]}}';
$up = mikhmon_onlogin_upgrade($sched, 'daily');
check_onlogin($up['changed'], 'scheduler-generation on-login must be upgraded');
check_onlogin(strpos($up['onlogin'], ':local mdays (31,28,') !== false, 'upgrade must adopt direct arithmetic');
check_onlogin(strpos($up['onlogin'], '/sys sch add') === false, 'upgrade must drop the temporary scheduler');
check_onlogin(strpos($up['onlogin'], ':put (",remc,1000,24h,2000,,Disable,,mikhmon,2312005,")') === 0, 'upgrade must preserve the metadata prefix verbatim');
check_onlogin(!mikhmon_onlogin_upgrade($up['onlogin'], 'daily')['changed'], 'upgrade must be idempotent');

// Original scheduler-generation format (legacy tail).
$legacy = ':put (",rem,1000,24h,2000,,Disable,"); '
    . '{:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; :local date [ /system clock get date ];:local year [ :pick $date 7 11 ];:local month [ :pick $date 0 3 ]; '
    . '/sys sch add name="$user" disable=no start-date=$date interval="24h"; ' . mikhmon_onlogin_legacy_tail() . '}}';
$leg = mikhmon_onlogin_upgrade($legacy, 'daily');
check_onlogin($leg['changed'], 'original scheduler generation must be upgraded');
check_onlogin(strpos($leg['onlogin'], ':local val "24h"') !== false, 'legacy upgrade must retain 24h validity');
check_onlogin(strpos($leg['onlogin'], ':local year 0; :local monum 0; :local day 0;') !== false, 'legacy upgrade must use the new parser');

// Self-computing format (no scheduler).
$self = ':put (",remc,1000,24h,2000,,Disable,"); '
    . '{:local date [ /system clock get date ]; :local exp [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; /ip hotspot user set comment="$date" [find where name="$user"]}}';
$sc = mikhmon_onlogin_upgrade($self, 'daily');
check_onlogin($sc['changed'], 'self-computing on-login must be rebuilt');

// Unknown customer script must be preserved.
$custom = ':log info "custom customer script"';
check_onlogin(!mikhmon_onlogin_upgrade($custom, 'daily')['changed'], 'unknown customer scripts must be preserved');

// Non-numeric price ("free") must survive, including the prefix rewrite.
$bfree = mikhmon_build_onlogin('remc', 'free', '60d', 'free', '', 'Disable', '');
check_onlogin(strpos($bfree['onlogin'], ':put (",remc,free,60d,free,,Disable,")') === 0, 'non-numeric price (free) must be preserved');
$freeOld = ':put (",remc,free,60d,free,,Disable,,mikhmon,2312005,"); {:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; :local schname ("exp-" . $user); /sys sch add name=$schname disable=no start-date=$date start-time=$time interval="60d"; :local firstexp ("$date $time");}}}';
$freeUp = mikhmon_onlogin_upgrade($freeOld, 'free-60-hari');
check_onlogin($freeUp['changed'], 'free-price profile must still be upgraded');
check_onlogin(strpos($freeUp['onlogin'], ':put (",remc,free,60d,free,,Disable,,mikhmon,2312005,")') === 0, 'free-price upgrade must keep prefix and extras');

/* --------------------------------------------------------------- monitor */

$mon = mikhmon_build_monitor('2000-4jam', 'remove');
check_onlogin(strpos($mon, '/ip hotspot user find where profile="2000-4jam"') !== false, 'monitor must filter the profile by name');
check_onlogin(strpos($mon, '/ip hotspot user remove $i') !== false, 'remove-mode monitor must remove expired users');
check_onlogin(strpos($mon, '/ip hotspot active remove [find where user=$name]') !== false, 'monitor must drop the expired active session');
check_onlogin(substr_count($mon, '{') === substr_count($mon, '}'), 'monitor braces must balance');
$monN = mikhmon_build_monitor('daily', 'set limit-uptime=1s');
check_onlogin(strpos($monN, '/ip hotspot user set limit-uptime=1s $i') !== false, 'notice-mode monitor must set limit-uptime');
check_onlogin(strpos(mikhmon_monitor_upgrade($mon), '[:pick $d 4 5] = "-"') !== false, 'built monitor must be ISO-clock compatible');

$monitor = ':local dateint do={:local montharray ( "jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec" );:local days [ :pick $d 4 6 ];}; :foreach i in [ /ip hotspot user find where profile="daily" ] do={}';
$updated = mikhmon_monitor_upgrade($monitor);
check_onlogin($updated !== $monitor && strpos($updated, '[:pick $d 4 5] = "-"') !== false, 'monitor must understand ISO router clock dates');
check_onlogin(mikhmon_monitor_upgrade($updated) === $updated, 'monitor upgrade must be idempotent');
check_onlogin(mikhmon_monitor_upgrade($custom) === $custom, 'monitor upgrade must preserve unknown scripts');

/* ------------------------------------- expiry arithmetic (logic parity) */

/*
 * Port of the RouterOS algorithm embedded by mikhmon_onlogin_body(). It proves
 * the day/year rollover (including leap years and midnight crossings) matches
 * PHP's own date math. RouterOS syntax itself still has to be run on a real
 * router, but the arithmetic is verified here.
 */
function ros_duration_seconds($v) {
    $total = 0; $num = ''; $len = strlen($v);
    for ($i = 0; $i < $len; $i++) {
        $ch = $v[$i];
        if (strpos('0123456789', $ch) !== false) { $num .= $ch; continue; }
        $n = ($num === '') ? 1 : (int) $num;
        if ($ch === 'd') { $total += $n * 86400; }
        if ($ch === 'h') { $total += $n * 3600; }
        if ($ch === 'm') { $total += $n * 60; }
        if ($ch === 's') { $total += $n; }
        if ($ch === 'w') { $total += $n * 604800; }
        $num = '';
    }
    return $total;
}
function ros_expiry($date, $time, $validity) {
    $ma = array("jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec");
    if (substr($date, 4, 1) === '-') {
        $year = (int) substr($date, 0, 4); $monum = (int) substr($date, 5, 2); $day = (int) substr($date, 8, 2);
    } else {
        $monum = array_search(substr($date, 0, 3), $ma) + 1; $day = (int) substr($date, 4, 2); $year = (int) substr($date, 7, 4);
    }
    $hh = (int) substr($time, 0, 2); $mi = (int) substr($time, 3, 2); $ss = (int) substr($time, 6, 2);
    $tot = $hh * 3600 + $mi * 60 + $ss + ros_duration_seconds($validity);
    $ad = 0; while ($tot >= 86400) { $tot -= 86400; $ad++; }
    $nh = 0; while ($tot >= 3600) { $tot -= 3600; $nh++; }
    $nmn = 0; while ($tot >= 60) { $tot -= 60; $nmn++; }
    $ns = $tot;
    $mdays = array(31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31);
    while ($ad > 0) {
        $dim = $mdays[$monum - 1];
        if ($monum == 2 && ($year % 4) == 0 && (($year % 100) != 0 || ($year % 400) == 0)) { $dim = 29; }
        $day++;
        if ($day > $dim) { $day = 1; $monum++; if ($monum > 12) { $monum = 1; $year++; } }
        $ad--;
    }
    return sprintf('%s/%02d/%d %02d:%02d:%02d', $ma[$monum - 1], $day, $year, $nh, $nmn, $ns);
}

function ros_to_php_relative($v) {
    $map = array('d' => 'days', 'h' => 'hours', 'm' => 'minutes', 's' => 'seconds', 'w' => 'weeks');
    $out = ''; $num = '';
    for ($i = 0; $i < strlen($v); $i++) {
        $ch = $v[$i];
        if (ctype_digit($ch)) { $num .= $ch; continue; }
        if (isset($map[$ch])) { $out .= ' ' . ($num === '' ? 1 : (int) $num) . ' ' . $map[$ch]; }
        $num = '';
    }
    return trim($out);
}

$cases = array(
    array('oct/18/2026', '22:00:00', '4h',  '2026-10-18 22:00:00'),
    array('oct/18/2026', '23:30:00', '1h',  '2026-10-18 23:30:00'),
    array('jan/31/2026', '23:00:00', '2h',  '2026-01-31 23:00:00'),
    array('feb/28/2024', '23:30:00', '1h',  '2024-02-28 23:30:00'),
    array('feb/28/2025', '10:00:00', '1d',  '2025-02-28 10:00:00'),
    array('dec/31/2026', '23:00:00', '2h',  '2026-12-31 23:00:00'),
    array('2026-10-18',  '08:15:30', '30m', '2026-10-18 08:15:30'),
    array('2026-03-01',  '00:00:00', '1d2h30m', '2026-03-01 00:00:00'),
    array('oct/18/2026', '12:00:00', '50h', '2026-10-18 12:00:00'),
    array('oct/18/2026', '12:00:00', '1w',  '2026-10-18 12:00:00'),
);
foreach ($cases as $c) {
    list($d, $t, $v, $base) = $c;
    $expected = strtolower(date('M/d/Y H:i:s', strtotime($base . ' ' . ros_to_php_relative($v))));
    $got = ros_expiry($d, $t, $v);
    check_onlogin($got === $expected, "expiry $d $t +$v => $got, expected $expected");
}

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "on-login regression checks passed\n";
