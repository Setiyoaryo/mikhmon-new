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
if (substr($_SERVER["REQUEST_URI"], -11) == "readcfg.php") {
    header("Location:./");
};
// read config

$iphost = explode('!', $data[$session][1])[1];
$userhost = explode('@|@', $data[$session][2])[1];
$passwdhost = explode('#|#', $data[$session][3])[1];
$hotspotname = explode('%', $data[$session][4])[1];
$dnsname = explode('^', $data[$session][5])[1];
$currency = explode('&', $data[$session][6])[1];
$areload = explode('*', $data[$session][7])[1];
$iface = explode('(', $data[$session][8])[1];
$infolp = explode(')', $data[$session][9])[1];
$idleto = explode('=', $data[$session][10])[1];
$sesname = explode('+', $data[$session][10])[1];
$useradm = explode('<|<', $data['mikhmon'][1])[1];
$passadm = explode('>|>', $data['mikhmon'][2])[1];
$livereport = explode('@!@', $data[$session][11])[1];

$cekindo['indo'] = array(
    'RP', 'Rp', 'rp', 'IDR', 'idr', 'RP.', 'Rp.', 'rp.', 'IDR.', 'idr.',
);



// -------------------------------------------------------------------------
// Helpers shared by the two pages that write include/config.php
// (settings/settings.php and settings/sessions.php).
//
// Every value is stored inside a single-quoted PHP string, one line per router
// session, and readcfg() reads it back by splitting on the delimiters above.
// A quote (or a backslash, or a newline) used to close that string early and
// leave config.php with a parse error - and a config.php that will not parse
// blanks the whole UI. Non-scalar input (a form field posted as an array) is
// rejected too, otherwise it would be stored as the literal "Array".
//
// Guarded with function_exists because readcfg.php is include()d from many
// pages and can therefore run more than once in a single request.
// -------------------------------------------------------------------------
if (!function_exists('mikhmon_cfg_clean')) {
    function mikhmon_cfg_clean($value)
    {
        if (!is_scalar($value)) {
            return '';
        }
        $value = (string) $value;
        $value = str_replace(array("'", '"', '<', '>', "\\", "\r", "\n", "\0"), '', $value);
        return trim($value);
    }
}

// Session names are used three ways: as the key in config.php, inside the
// delimited values on that line, and inside an inline <script> redirect. Keep
// them identifier-like so none of those can be broken from a pasted value.
if (!function_exists('mikhmon_cfg_name')) {
    function mikhmon_cfg_name($value)
    {
        $value = preg_replace('/\s+/', '-', mikhmon_cfg_clean($value));
        $value = preg_replace('/[^A-Za-z0-9._-]/', '', $value);
        return trim($value, '-');
    }
}

// Write a config file through a unique temporary file, then rename over the
// target. A reader (or a request that includes the file) either sees the old
// content or the new one, never a truncated file, and two concurrent saves
// cannot publish each other's half written output.
if (!function_exists('mikhmon_cfg_write')) {
    function mikhmon_cfg_write($file, $content)
    {
        $tmp = $file . '.' . getmypid() . '.' . mt_rand(100000, 999999) . '.tmp';

        if (@file_put_contents($tmp, $content) !== false) {
            if (@rename($tmp, $file)) {
                return true;
            }
            // Some layouts refuse to rename over the target - a config file
            // that is itself a bind mount, for instance - so fall back to
            // writing in place rather than losing the save.
            @unlink($tmp);
        }

        return @file_put_contents($file, $content) !== false;
    }
}
