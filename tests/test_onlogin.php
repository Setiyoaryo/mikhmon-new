<?php
require __DIR__ . '/../include/onlogin.php';

$failures = array();
function check_onlogin($condition, $message) {
    global $failures;
    if (!$condition) { $failures[] = $message; }
}

$script = mikhmon_build_onlogin('rem', '1000', '24h', '2000', '', 'Disable', '')['onlogin'];
check_onlogin(strpos($script, 'start-time=$time interval="24h"') !== false, '24h must be anchored to the first login time');
check_onlogin(strpos($script, '$exp = $firstexp') !== false, 'must reject next-run equal to login time');
check_onlogin(strpos($script, '[:find $exp "/"] < 0') === false, 'RouterOS find returns nil, not a negative index');
check_onlogin(strpos($script, '[:len $date] = 10') !== false, 'ISO clock date must yield the actual year');
check_onlogin(strpos($script, 'comment="$date $exp"') === false, 'time-only next-run must normalize an ISO clock date before saving expiry');

// Previously considered current, even though next-run parsing was unsafe.
$old = ':put (",rem,1000,24h,2000,,Disable,"); {:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; '
    . ':local ucode [:pic $comment 0 2]; :if ($ucode = "vc" or $ucode = "up" or $comment = "") do={ :local date [ /system clock get date ]; '
    . ':local time [ /system clock get time ]; :local year [ :pick $date 7 11 ]; '
    . ':local month [ :pick $date 0 3 ]; :local schname ("exp-" . $user); '
    . '/sys sch add name=$schname disable=no start-date=$date start-time=$time interval="24h"; '
    . ':local exp [ /sys sch get [ /sys sch find where name=$schname ] next-run]; '
    . ':if ([:find $exp "/"] < 0 and [:find $exp "-"] < 0) do={}; /sys sch remove [find where name=$schname]}}';
$upgrade = mikhmon_onlogin_upgrade($old, 'daily');
check_onlogin($upgrade['changed'], 'Fix Validity must upgrade the previous exp- scheduler generation');
check_onlogin(strpos($upgrade['onlogin'], 'interval="24h"') !== false, 'upgrade must retain 24h validity');
check_onlogin(strpos($upgrade['onlogin'], ':put (",rem,1000,24h,2000,,Disable,")') === 0, 'upgrade must preserve pricing metadata');
$again = mikhmon_onlogin_upgrade($upgrade['onlogin'], 'daily');
check_onlogin(!$again['changed'], 'upgrade must be idempotent');
$lock = '; [:local mac $"mac-address"; /ip hotspot user set mac-address=$mac [find where name=$user]]';
$record = '; /system script add name="sale-record" comment="mikhmon"';
$withExtras = str_replace('}}', $record . $lock . '}}', $old);
$extrasUpgrade = mikhmon_onlogin_upgrade($withExtras, 'daily');
check_onlogin(strpos($extrasUpgrade['onlogin'], $record . $lock) !== false, 'upgrade must preserve record and MAC-lock actions');
$legacy = ':put (",rem,1000,24h,2000,,Disable,"); '
    . '{:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; '
    . ':local date [ /system clock get date ];:local year [ :pick $date 7 11 ];:local month [ :pick $date 0 3 ]; '
    . '/sys sch add name="$user" disable=no start-date=$date interval="24h"; '
    . mikhmon_onlogin_legacy_tail() . '}}';
$legacyUpgrade = mikhmon_onlogin_upgrade($legacy, 'daily');
check_onlogin($legacyUpgrade['changed'], 'original scheduler generation must be upgraded');
check_onlogin(strpos($legacyUpgrade['onlogin'], 'start-time=$time interval="24h"') !== false, 'legacy upgrade must anchor 24h to login');
check_onlogin(strpos($legacyUpgrade['onlogin'], ':local firstexp') !== false, 'legacy upgrade must use the current next-run parser');
check_onlogin(strpos($legacyUpgrade['onlogin'], ':set year [:pick $date 0 4]') !== false, 'legacy upgrade must read the ISO clock year');
$custom = ':log info "custom customer script"';
check_onlogin(!mikhmon_onlogin_upgrade($custom, 'daily')['changed'], 'unknown customer scripts must be preserved');

if (function_exists('mikhmon_monitor_upgrade')) {
    $monitor = ':local dateint do={:local montharray ( "jan","feb","mar","apr","may","jun","jul","aug","sep","oct","nov","dec" );:local days [ :pick $d 4 6 ];}; :foreach i in [ /ip hotspot user find where profile="daily" ] do={}';
    $updated = mikhmon_monitor_upgrade($monitor);
    check_onlogin($updated !== $monitor && strpos($updated, '[:pick $d 4 5] = "-"') !== false, 'monitor must understand ISO router clock dates');
    check_onlogin(mikhmon_monitor_upgrade($updated) === $updated, 'monitor upgrade must be idempotent');
    check_onlogin(mikhmon_monitor_upgrade($custom) === $custom, 'monitor upgrade must preserve unknown scripts');
} else {
    check_onlogin(false, 'profile monitor needs a shared ISO-date upgrade');
}

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "on-login regression checks passed\n";
