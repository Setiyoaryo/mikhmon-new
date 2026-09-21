<?php
// End-to-end integration test for Mikhmon Voucher Generator

$mockPort = 18729;
$mockAddr = "127.0.0.1:" . $mockPort;

// 1. Start mock RouterOS API server in background
$mockCmd = __DIR__ . '/../bin/voucher-generator -mock-server ' . $mockAddr;
$descriptorspec = array(
    0 => array("pipe", "r"),
    1 => array("pipe", "w"),
    2 => array("pipe", "w")
);
$mockProcess = proc_open($mockCmd, $descriptorspec, $pipes);
if (!is_resource($mockProcess)) {
    echo "FAIL: could not start mock server\n";
    exit(1);
}

// Wait briefly for server to bind
usleep(150000);

echo "Mock server started on $mockAddr\n";

// 2. Test generating 5000 vouchers via bin/voucher-generator
$go_bin = __DIR__ . '/../bin/voucher-generator';
$payload = json_encode(array(
    "host" => "127.0.0.1",
    "port" => $mockPort,
    "user" => "admin",
    "pass" => "secret",
    "ssl" => false,
    "qty" => 5000,
    "server" => "all",
    "mode" => "vc",
    "userl" => 5,
    "prefix" => "VC5K-",
    "char" => "mix",
    "profile" => "default",
    "timelimit" => "1h",
    "datalimit" => 104857600,
    "comment" => "vc-integration-5000",
    "concurrency" => 16,
    "timeout" => 10
));

$start = microtime(true);
$process = proc_open($go_bin . " -stdin", $descriptorspec, $clientPipes);
if (!is_resource($process)) {
    echo "FAIL: could not execute $go_bin\n";
    proc_terminate($mockProcess);
    exit(1);
}

fwrite($clientPipes[0], $payload);
fclose($clientPipes[0]);
$stdout = stream_get_contents($clientPipes[1]);
fclose($clientPipes[1]);
$stderr = stream_get_contents($clientPipes[2]);
fclose($clientPipes[2]);
$status = proc_close($process);
$duration = round((microtime(true) - $start) * 1000, 2);

echo "Generator stdout: $stdout\n";
if (!empty($stderr)) {
    echo "Generator stderr: $stderr\n";
}

$res = json_decode($stdout, true);
if (!$res || !isset($res['success']) || $res['success'] !== true) {
    echo "FAIL: Generator did not return success!\n";
    proc_terminate($mockProcess);
    exit(1);
}

if ($res['count'] !== 5000) {
    echo "FAIL: Expected count 5000, got " . $res['count'] . "\n";
    proc_terminate($mockProcess);
    exit(1);
}

echo "SUCCESS: 5000 vouchers generated in {$duration} ms (Go reported {$res['duration_ms']} ms)!\n";
echo "First user: " . $res['first_user'] . "\n";

// 3. Test generating 1 voucher in "up" mode
$payload1 = json_encode(array(
    "host" => "127.0.0.1",
    "port" => $mockPort,
    "user" => "admin",
    "pass" => "secret",
    "ssl" => false,
    "qty" => 1,
    "server" => "all",
    "mode" => "up",
    "userl" => 4,
    "prefix" => "U1-",
    "char" => "lower",
    "profile" => "default",
    "timelimit" => "0",
    "datalimit" => 0,
    "comment" => "up-1-test",
    "concurrency" => 1,
    "timeout" => 5
));

$process1 = proc_open($go_bin . " -stdin", $descriptorspec, $clientPipes1);
fwrite($clientPipes1[0], $payload1);
fclose($clientPipes1[0]);
$stdout1 = stream_get_contents($clientPipes1[1]);
fclose($clientPipes1[1]);
$stderr1 = stream_get_contents($clientPipes1[2]);
fclose($clientPipes1[2]);
proc_close($process1);

$res1 = json_decode($stdout1, true);
if (!$res1 || !$res1['success'] || $res1['count'] !== 1) {
    echo "FAIL: Single voucher generation failed!\n";
    proc_terminate($mockProcess);
    exit(1);
}

echo "SUCCESS: Single voucher generation passed! First user: " . $res1['first_user'] . "\n";

// Clean up mock process
proc_terminate($mockProcess);
proc_close($mockProcess);

echo "ALL INTEGRATION TESTS PASSED!\n";
