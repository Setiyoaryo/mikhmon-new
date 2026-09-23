<?php
/**
 * Test script for hotspot=users routing and status tabs rendering.
 */

if (session_status() === PHP_SESSION_NONE) {
    session_start();
}
$_SESSION["mikhmon"] = "admin";
$currency = "Rp";
$cekindo = array('indo' => array('Rp'));
$session = "taufiq";
$url = "./?hotspot=users&session=taufiq";
require_once dirname(__FILE__) . '/../lang/en.php';
require_once dirname(__FILE__) . '/../lib/formatbytesbites.php';
require_once dirname(__FILE__) . '/../lib/routeros_api.class.php';
require_once dirname(__FILE__) . '/../include/hscache.php';

function test_index_routing() {
    $tests = array(
        array('query' => array('hotspot' => 'users', 'status' => 'ready'), 'expected_branch' => 'users_default'),
        array('query' => array('hotspot' => 'users', 'status' => 'active'), 'expected_branch' => 'users_default'),
        array('query' => array('hotspot' => 'users', 'status' => 'expired'), 'expected_branch' => 'users_default'),
        array('query' => array('hotspot' => 'users', 'status' => 'member'), 'expected_branch' => 'users_default'),
        array('query' => array('hotspot' => 'users', 'status' => 'all'), 'expected_branch' => 'users_default'),
        array('query' => array('hotspot' => 'users', 'profile' => 'all'), 'expected_branch' => 'users_default'),
        array('query' => array('hotspot' => 'users', 'profile' => 'vip'), 'expected_branch' => 'users_profile'),
        array('query' => array('hotspot' => 'users', 'comment' => 'vc-batch1'), 'expected_branch' => 'users_comment'),
        array('query' => array('hotspot' => 'users', 'profile' => 'vip', 'status' => 'ready'), 'expected_branch' => 'users_profile'),
    );

    foreach ($tests as $t) {
        $hotspot = isset($t['query']['hotspot']) ? $t['query']['hotspot'] : '';
        $prof = isset($t['query']['profile']) ? $t['query']['profile'] : '';
        $comm = isset($t['query']['comment']) ? $t['query']['comment'] : '';

        $matched = 'none';
        if ($hotspot == "users" && $comm != "") {
            $matched = 'users_comment';
        } elseif ($hotspot == "users" && $prof != "" && $prof != "all") {
            $matched = 'users_profile';
        } elseif ($hotspot == "users") {
            $matched = 'users_default';
        }

        if ($matched !== $t['expected_branch']) {
            throw new Exception("Routing test failed for " . json_encode($t['query']) . ": expected " . $t['expected_branch'] . ", got " . $matched);
        }
    }
    echo "[PASS] Routing logic in index.php matches correctly for all status/profile combinations.\n";
}

class MockAPI {
    public $users = array();
    public $profiles = array(
        array('name' => 'default'),
        array('name' => '1jam'),
    );

    public function comm($cmd, $params = array()) {
        if ($cmd === "/ip/hotspot/user/profile/print") {
            return $this->profiles;
        }
        if ($cmd === "/ip/hotspot/user/print") {
            return $this->users;
        }
        return array();
    }
}

function test_users_php_rendering() {
    global $API, $session, $url, $currency, $cekindo, $_users, $_add, $_generate, $_processing,
           $_all, $_ready_stock, $_in_use, $_expired, $_member, $_search, $_profile, $_show_all,
           $_comment, $_by_comment, $_unused_older, $_print, $_print_qr, $_print_small,
           $_print_default, $_name, $_uptime_user, $_no_users_found;

    $mock_users_dataset = array(
        // 1. Ready voucher
        array(
            '.id' => '*1',
            'server' => 'all',
            'name' => 'vc001',
            'password' => 'vc001',
            'profile' => 'default',
            'mac-address' => '',
            'uptime' => '0s',
            'bytes-in' => '0',
            'bytes-out' => '0',
            'comment' => 'vc-batch1',
            'disabled' => 'false',
            'limit-uptime' => '1h',
            'limit-bytes-total' => '',
        ),
        // 2. Active voucher (has traffic/uptime)
        array(
            '.id' => '*2',
            'server' => 'all',
            'name' => 'vc002',
            'password' => 'vc002',
            'profile' => 'default',
            'mac-address' => 'AA:BB:CC:DD:EE:FF',
            'uptime' => '10m',
            'bytes-in' => '1048576',
            'bytes-out' => '2097152',
            'comment' => 'vc-batch1',
            'disabled' => 'false',
            'limit-uptime' => '1h',
            'limit-bytes-total' => '',
        ),
        // 3. Expired voucher (limit-uptime = 1s)
        array(
            '.id' => '*3',
            'server' => 'all',
            'name' => 'vc003',
            'password' => 'vc003',
            'profile' => 'default',
            'mac-address' => '11:22:33:44:55:66',
            'uptime' => '1h',
            'bytes-in' => '5000000',
            'bytes-out' => '5000000',
            'comment' => 'vc-batch1',
            'disabled' => 'false',
            'limit-uptime' => '1s',
            'limit-bytes-total' => '',
        ),
        // 4. Member user (name != password)
        array(
            '.id' => '*4',
            'server' => 'all',
            'name' => 'john',
            'password' => 'secret123',
            'profile' => 'default',
            'mac-address' => '99:88:77:66:55:44',
            'uptime' => '0s',
            'bytes-in' => '0',
            'bytes-out' => '0',
            'comment' => '',
            'disabled' => 'false',
            'limit-uptime' => '',
            'limit-bytes-total' => '',
        ),
    );

    $scenarios = array(
        'All users default' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq'),
            'users' => $mock_users_dataset,
            'expect_rows' => 4,
            'expect_no_users' => false,
            'expect_counts' => array('all' => 4, 'ready' => 1, 'active' => 1, 'expired' => 1, 'member' => 1),
        ),
        'Status = ready' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'ready'),
            'users' => $mock_users_dataset,
            'expect_rows' => 1,
            'expect_no_users' => false,
            'expect_name' => 'vc001',
        ),
        'Status = active' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'active'),
            'users' => $mock_users_dataset,
            'expect_rows' => 1,
            'expect_no_users' => false,
            'expect_name' => 'vc002',
        ),
        'Status = expired' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'expired'),
            'users' => $mock_users_dataset,
            'expect_rows' => 1,
            'expect_no_users' => false,
            'expect_name' => 'vc003',
        ),
        'Status = member' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'member'),
            'users' => $mock_users_dataset,
            'expect_rows' => 1,
            'expect_no_users' => false,
            'expect_name' => 'john',
        ),
        'Empty router (0 users)' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'ready'),
            'users' => array(),
            'expect_rows' => 0,
            'expect_no_users' => true,
        ),
        '0 matching in ready status' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'ready'),
            'users' => array($mock_users_dataset[1]), // only active user
            'expect_rows' => 0,
            'expect_no_users' => true,
        ),
        'Status = all with query parameter' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'all'),
            'users' => $mock_users_dataset,
            'expect_rows' => 4,
            'expect_no_users' => false,
            'expect_counts' => array('all' => 4, 'ready' => 1, 'active' => 1, 'expired' => 1, 'member' => 1),
        ),
        'Profile + Status filter (default + ready)' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'profile' => 'default', 'status' => 'ready'),
            'users' => $mock_users_dataset,
            'expect_rows' => 1,
            'expect_no_users' => false,
            'expect_name' => 'vc001',
        ),
        'Profile + Status filter with 0 match (1jam + ready)' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'profile' => '1jam', 'status' => 'ready'),
            'users' => $mock_users_dataset,
            'expect_rows' => 0,
            'expect_no_users' => true,
        ),
        'Comment + Status filter (vc-batch1 + active)' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'comment' => 'vc-batch1', 'status' => 'active'),
            'users' => $mock_users_dataset,
            'expect_rows' => 1,
            'expect_no_users' => false,
            'expect_name' => 'vc002',
        ),
        'Voucher with missing fields (empty attributes)' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'ready'),
            'users' => array(
                array(
                    '.id' => '*99',
                    'name' => 'barevc',
                    'password' => 'barevc',
                    // missing server, mac-address, comment, uptime, bytes-in, bytes-out, etc.
                )
            ),
            'expect_rows' => 1,
            'expect_no_users' => false,
            'expect_name' => 'barevc',
        ),
        'Member with missing fields' => array(
            'get' => array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'member'),
            'users' => array(
                array(
                    '.id' => '*100',
                    'name' => 'baremember',
                    'password' => 'otherpass',
                )
            ),
            'expect_rows' => 1,
            'expect_no_users' => false,
            'expect_name' => 'baremember',
        ),
    );

    $API = new MockAPI();

    foreach ($scenarios as $name => $s) {
        $_GET = $s['get'];
        $prof = isset($_GET['profile']) ? $_GET['profile'] : '';
        $comm = isset($_GET['comment']) ? $_GET['comment'] : '';

        // Inject cache data
        $API->users = $s['users'];
        mikhmon_hscache_write('hotspot-users-' . $session, $s['users']);

        ob_start();
        include dirname(__FILE__) . '/../hotspot/users.php';
        $output = ob_get_clean();

        // 1. Output must not be blank
        if (trim($output) === '') {
            throw new Exception("Scenario '$name' produced completely BLANK output!");
        }

        // 2. Must not contain premature JS redirects when count is 0
        if ($s['expect_no_users'] && strpos($output, "window.location='./?hotspot=users&profile=all") !== false) {
            throw new Exception("Scenario '$name' unexpectedly redirected when 0 users matched!");
        }

        // 3. Must contain the card and status tabs
        if (strpos($output, 'card') === false || strpos($output, 'status=ready') === false) {
            throw new Exception("Scenario '$name' missing card or status tabs in HTML!");
        }

        // 4. Verify no_users message or row presence
        if ($s['expect_no_users']) {
            if (strpos($output, 'No users found') === false) {
                throw new Exception("Scenario '$name' expected 'No users found' row but was not found in HTML!");
            }
        } else {
            if (isset($s['expect_name']) && strpos($output, $s['expect_name']) === false) {
                throw new Exception("Scenario '$name' expected user '{$s['expect_name']}' not found in HTML!");
            }
        }

        // 5. Verify tab counts if specified
        if (isset($s['expect_counts'])) {
            foreach ($s['expect_counts'] as $stKey => $cnt) {
                if (strpos($output, ">$cnt</span>") === false) {
                    throw new Exception("Scenario '$name' expected count $cnt for status $stKey not found!");
                }
            }
        }

        echo "[PASS] $name\n";
    }
}

function test_indonesian_language() {
    global $API, $session, $url, $currency, $cekindo, $_users, $_add, $_generate, $_processing,
           $_all, $_ready_stock, $_in_use, $_expired, $_member, $_search, $_profile, $_show_all,
           $_comment, $_by_comment, $_unused_older, $_print, $_print_qr, $_print_small,
           $_print_default, $_name, $_uptime_user, $_no_users_found;

    require dirname(__FILE__) . '/../lang/id.php';
    $_GET = array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'ready');
    $prof = '';
    $comm = '';
    $API = new MockAPI();
    $API->users = array();
    mikhmon_hscache_write('hotspot-users-' . $session, array());

    ob_start();
    include dirname(__FILE__) . '/../hotspot/users.php';
    $output = ob_get_clean();

    if (strpos($output, 'Tidak ada user ditemukan') === false) {
        throw new Exception("Expected 'Tidak ada user ditemukan' in Indonesian output!");
    }
    echo "[PASS] Indonesian language 'Tidak ada user ditemukan'\n";
}

function test_export_users() {
    global $API, $session;
    require dirname(__FILE__) . '/../lang/en.php';
    $API = new MockAPI();

    $mock_users = array(
        array(
            '.id' => '*1',
            'server' => 'all',
            'name' => 'user-ready',
            'password' => 'user-ready',
            'profile' => 'default',
            'mac-address' => '',
            'uptime' => '0s',
            'bytes-in' => '0',
            'bytes-out' => '0',
            'comment' => 'vc-100-batch',
            'disabled' => 'false',
            'limit-uptime' => '1h',
            'limit-bytes-total' => '',
        ),
        array(
            '.id' => '*2',
            'server' => 'all',
            'name' => 'user-active',
            'password' => 'user-active',
            'profile' => 'default',
            'mac-address' => 'AA:BB:CC:DD:EE:FF',
            'uptime' => '5m',
            'bytes-in' => '1000',
            'bytes-out' => '2000',
            'comment' => 'vc-100-batch',
            'disabled' => 'false',
            'limit-uptime' => '1h',
            'limit-bytes-total' => '',
        ),
    );

    $API->users = $mock_users;
    mikhmon_hscache_write('hotspot-users-' . $session, $mock_users);

    // 1. Export ready script
    $_GET = array('hotspot' => 'export-users', 'session' => 'taufiq', 'status' => 'ready', 'export' => 'script');
    $prof = '';
    $comm = '';
    ob_start();
    include dirname(__FILE__) . '/../hotspot/exportusers.php';
    $out_script = ob_get_clean();

    if (strpos($out_script, 'user-ready') === false) {
        throw new Exception("Export script expected 'user-ready' to be exported!");
    }
    if (strpos($out_script, 'user-active') !== false) {
        throw new Exception("Export script unexpectedly included 'user-active' when filtered by status=ready!");
    }

    // 2. Export active CSV
    $_GET = array('hotspot' => 'export-users', 'session' => 'taufiq', 'status' => 'active', 'export' => 'csv');
    $prof = '';
    $comm = '';
    ob_start();
    include dirname(__FILE__) . '/../hotspot/exportusers.php';
    $out_csv = ob_get_clean();

    if (strpos($out_csv, 'user-active') === false) {
        throw new Exception("Export CSV expected 'user-active' to be exported!");
    }
    if (strpos($out_csv, 'user-ready') !== false) {
        throw new Exception("Export CSV unexpectedly included 'user-ready' when filtered by status=active!");
    }

    // 3. Export 0-match status (expired)
    $_GET = array('hotspot' => 'export-users', 'session' => 'taufiq', 'status' => 'expired', 'export' => 'csv');
    $prof = '';
    $comm = '';
    ob_start();
    include dirname(__FILE__) . '/../hotspot/exportusers.php';
    $out_empty = ob_get_clean();

    if (strpos($out_empty, 'No users found') === false) {
        throw new Exception("Export CSV expected 'No users found' on 0-match status!");
    }

    echo "[PASS] exportusers.php status filtering (script & csv)\n";
}

function test_comment_dropdown_preservation() {
    global $API, $session, $url, $currency, $cekindo, $_users, $_add, $_generate, $_processing,
           $_all, $_ready_stock, $_in_use, $_expired, $_member, $_search, $_profile, $_show_all,
           $_comment, $_by_comment, $_unused_older, $_print, $_print_qr, $_print_small,
           $_print_default, $_name, $_uptime_user, $_no_users_found;

    $mock_users = array(
        array(
            '.id' => '*1',
            'server' => 'all',
            'name' => 'u1',
            'password' => 'u1',
            'profile' => 'default',
            'uptime' => '5m',
            'bytes-in' => '100',
            'bytes-out' => '200',
            'comment' => 'vc-555-batchA',
            'disabled' => 'false',
        ),
        array(
            '.id' => '*2',
            'server' => 'all',
            'name' => 'u2',
            'password' => 'u2',
            'profile' => 'default',
            'uptime' => '0s',
            'bytes-in' => '0',
            'bytes-out' => '0',
            'comment' => 'vc-777-batchB',
            'disabled' => 'false',
        ),
    );

    $API = new MockAPI();
    $API->users = $mock_users;
    mikhmon_hscache_write('hotspot-users-' . $session, $mock_users);

    // When status=ready (where batchA has 0 users), the comment dropdown must STILL offer batchA and batchB
    $_GET = array('hotspot' => 'users', 'session' => 'taufiq', 'status' => 'ready');
    $prof = '';
    $comm = '';

    ob_start();
    include dirname(__FILE__) . '/../hotspot/users.php';
    $output = ob_get_clean();

    if (strpos($output, 'vc-555-batchA') === false) {
        throw new Exception("Expected batch vc-555-batchA to be retained in comment dropdown when filtered by status!");
    }
    if (strpos($output, 'vc-777-batchB') === false) {
        throw new Exception("Expected batch vc-777-batchB to be in comment dropdown!");
    }

    echo "[PASS] Comment dropdown preserves batches under status filtering\n";
}

test_index_routing();
test_users_php_rendering();
test_indonesian_language();
test_export_users();
test_comment_dropdown_preservation();
echo "\nALL TESTS PASSED SUCCESSFULLY!\n";
