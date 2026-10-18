<?php
require __DIR__ . '/../include/onlogin.php';

$failures = array();
function check_onlogin($condition, $message) {
    global $failures;
    if (!$condition) { $failures[] = $message; }
}

/* ---------------------------------------------------------------- build */

$script = mikhmon_build_onlogin('rem', '1000', '24h', '2000', '', 'Disable', '')['onlogin'];
check_onlogin(strpos($script, 'start-time=$time interval="24h"') !== false, '24h must be anchored to the first login time');
check_onlogin(strpos($script, '$schname') !== false, 'must use a unique exp- scheduler name');
check_onlogin(strpos($script, ':local firstexp') !== false, 'must reject next-run equal to login time');
check_onlogin(strpos($script, ':error') === false, 'must never abort the hotspot login with a fatal error');
check_onlogin(strpos($script, ':local mdays') === false, 'must not use the arithmetic that v6.49 will not run');
check_onlogin(strpos($script, ':local val "24h"') === false, 'scheduler method embeds validity in the interval, not as a literal');

$notice = mikhmon_build_onlogin('ntf', '1000', '4h', '2000', '', 'Disable', '')['onlogin'];
check_onlogin(strpos($notice, 'start-time=$time interval="4h"') !== false, 'notice mode keeps the scheduler');
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

// Arithmetic generation (deployed during the failed attempt) must be rebuilt.
$arith = ':put (",remc,1000,24h,2000,,Disable,,mikhmon,2312005,"); '
    . '{:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; :local ucode [:pic $comment 0 2]; '
    . ':if ($ucode = "vc" or $ucode = "up" or $comment = "") do={ :local mdays (31,28,31,30,31,30,31,31,30,31,30,31); ; }}';
$up = mikhmon_onlogin_upgrade($arith, 'daily');
check_onlogin($up['changed'], 'arithmetic-generation on-login must be upgraded');
check_onlogin(strpos($up['onlogin'], 'start-time=$time interval="24h"') !== false, 'upgrade must adopt the scheduler method');
check_onlogin(strpos($up['onlogin'], ':local mdays') === false, 'upgrade must drop the arithmetic');
check_onlogin(strpos($up['onlogin'], ':put (",remc,1000,24h,2000,,Disable,,mikhmon,2312005,")') === 0, 'upgrade must preserve the metadata prefix verbatim');
check_onlogin(!mikhmon_onlogin_upgrade($up['onlogin'], 'daily')['changed'], 'upgrade must be idempotent');

// Original scheduler-generation format (legacy tail).
$legacy = ':put (",rem,1000,24h,2000,,Disable,"); '
    . '{:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; :local date [ /system clock get date ];:local year [ :pick $date 7 11 ];:local month [ :pick $date 0 3 ]; '
    . '/sys sch add name="$user" disable=no start-date=$date interval="24h"; ' . mikhmon_onlogin_legacy_tail() . '}}';
$leg = mikhmon_onlogin_upgrade($legacy, 'daily');
check_onlogin($leg['changed'], 'original scheduler generation must be upgraded');
check_onlogin(strpos($leg['onlogin'], 'start-time=$time interval="24h"') !== false, 'legacy upgrade must anchor 24h to login');
check_onlogin(strpos($leg['onlogin'], '$schname') !== false, 'legacy upgrade must use the unique scheduler name');

// Unknown customer script must be preserved.
$custom = ':log info "custom customer script"';
check_onlogin(!mikhmon_onlogin_upgrade($custom, 'daily')['changed'], 'unknown customer scripts must be preserved');

// Non-numeric price ("free") must survive, including the prefix rewrite.
$bfree = mikhmon_build_onlogin('remc', 'free', '60d', 'free', '', 'Disable', '');
check_onlogin(strpos($bfree['onlogin'], ':put (",remc,free,60d,free,,Disable,")') === 0, 'non-numeric price (free) must be preserved');
$freeOld = ':put (",remc,free,60d,free,,Disable,,mikhmon,2312005,"); {:local comment [ /ip hotspot user get [/ip hotspot user find where name="$user"] comment]; :local schname ("exp-" . $user); /sys sch add name=$schname disable=no start-date=$date start-time=$time interval="60d"; :local firstexp ("$date $time");}}}';
$freeUp = mikhmon_onlogin_upgrade($freeOld, 'free-60-hari');
check_onlogin($freeUp['changed'] === false, 'already-upgraded scheduler profile must be skipped');
check_onlogin(strpos($freeUp['onlogin'], ':put (",remc,free,60d,free,,Disable,,mikhmon,2312005,")') === 0, 'skipped profile keeps its prefix');

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

if ($failures) {
    fwrite(STDERR, implode("\n", $failures) . "\n");
    exit(1);
}
echo "on-login regression checks passed\n";
