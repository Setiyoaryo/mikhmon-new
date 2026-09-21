<?php
/*****************************
 *
 * RouterOS PHP API class v1.6 — Mikhmon Go backend bridge
 *
 * Author: Denis Basta
 * Contributors:
 *    Nick Barnes
 *    Ben Menking (ben [at] infotechsc [dot] com)
 *    Jeremy Jefferson (http://jeremyj.com)
 *    Cristian Deluxe (djcristiandeluxe [at] gmail [dot] com)
 *    Mikhail Moskalev (mmv.rus [at] gmail [dot] com)
 *
 * http://www.mikrotik.com
 * http://wiki.mikrotik.com/wiki/API_PHP_class
 *
 ******************************
 *
 * This build keeps the exact public surface of the original class
 * (connect / disconnect / comm / write / read / parseResponse / debug and the
 * encrypt, decrypt, rand* helpers) but does not open a TCP socket to the
 * router itself.
 *
 * Every call is forwarded over HTTP to the `mikhmon-api` Go service, which
 * owns a pool of authenticated RouterOS connections. The wire format is
 * unchanged: the Go service returns the same raw sentences a socket read would
 * have produced, so parseResponse() below still does all the parsing and every
 * caller in the application behaves exactly as before.
 *
 * Why: the PHP version dialled and logged in to the router once per request and
 * pushed generated vouchers in a single sequential loop. The Go service reuses
 * connections across requests and can push a whole batch in parallel.
 *
 ******************************/

class RouterosAPI
{
    var $debug     = false; //  Show debug information
    var $connected = false; //  Connection state
    var $port      = 8728;  //  Port to connect to (default 8729 for ssl)
    var $ssl       = false; //  Connect using SSL (must enable api-ssl in IP/Services)
    var $timeout   = 3;     //  Connection attempt timeout and data read timeout
    var $attempts  = 5;     //  Connection attempt count
    var $delay     = 3;     //  Delay between connection attempts in seconds

    var $socket;            //  Kept for backwards compatibility, unused
    var $error_no;          //  Variable for storing connection error number, if any
    var $error_str;         //  Variable for storing connection error text, if any

    var $session   = null;  //  Session id handed out by the Go backend
    var $sentences = array(); // Completed sentences waiting for a read()
    var $pending   = array(); // Words of the sentence currently being built

    /* Check, can be var used in foreach  */
    public function isIterable($var)
    {
        return $var !== null
                && (is_array($var)
                || $var instanceof Traversable
                || $var instanceof Iterator
                || $var instanceof IteratorAggregate
                );
    }

    /**
     * Print text for debug purposes
     *
     * @param string      $text       Text to print
     *
     * @return void
     */
    public function debug($text)
    {
        if ($this->debug) {
            echo $text . "\n";
        }
    }

    /**
     * Kept for compatibility with the original socket implementation.
     *
     * @param string        $length
     *
     * @return void
     */
    public function encodeLength($length)
    {
        if ($length < 0x80) {
            $length = chr($length);
        } elseif ($length < 0x4000) {
            $length |= 0x8000;
            $length = chr(($length >> 8) & 0xFF) . chr($length & 0xFF);
        } elseif ($length < 0x200000) {
            $length |= 0xC00000;
            $length = chr(($length >> 16) & 0xFF) . chr(($length >> 8) & 0xFF) . chr($length & 0xFF);
        } elseif ($length < 0x10000000) {
            $length |= 0xE0000000;
            $length = chr(($length >> 24) & 0xFF) . chr(($length >> 16) & 0xFF) . chr(($length >> 8) & 0xFF) . chr($length & 0xFF);
        } elseif ($length >= 0x10000000) {
            $length = chr(0xF0) . chr(($length >> 24) & 0xFF) . chr(($length >> 16) & 0xFF) . chr(($length >> 8) & 0xFF) . chr($length & 0xFF);
        }

        return $length;
    }


    /**
     * Login to RouterOS
     *
     * Asks the Go backend to dial the router, authenticate and keep the
     * connection in its pool. The pooled connection is then reused by every
     * later comm() call, including those made by subsequent HTTP requests.
     *
     * @param string      $ip         Hostname (IP or domain) of the RouterOS server
     * @param string      $login      The RouterOS username
     * @param string      $password   The RouterOS password
     *
     * @return boolean                If we are connected or not
     */
    public function connect($ip, $login, $password)
    {
        $this->session   = null;
        $this->sentences = array();
        $this->pending   = array();
        $this->connected = false;
        $this->error_no  = 0;
        $this->error_str = '';

        $this->debug('Connection attempt to ' . ($this->ssl ? 'ssl://' : '') . $ip . ':' . $this->port . '...');

        $response = mikhmon_api_post('/v1/connect', array(
            'host'       => $ip,
            'port'       => (int) $this->port,
            'user'       => $login,
            'pass'       => $password,
            'ssl'        => (bool) $this->ssl,
            'timeout_ms' => $this->timeoutMs(),
        ), mikhmon_api_connect_timeout());

        if (is_array($response) && !empty($response['ok']) && !empty($response['session'])) {
            $this->session   = $response['session'];
            $this->connected = true;
            $this->debug('Connected...');
        } else {
            $this->error_str = mikhmon_api_error($response);
            $this->debug('Error... ' . $this->error_str);
        }

        return $this->connected;
    }


    /**
     * Disconnect from RouterOS
     *
     * The socket is intentionally left in the Go pool: it stays warm for the
     * next request. Only the local session state is cleared.
     *
     * @return void
     */
    public function disconnect()
    {
        $this->flushPending();
        $this->connected = false;
        $this->session   = null;
        $this->sentences = array();
        $this->pending   = array();
        $this->debug('Disconnected...');
    }


    /**
     * Parse response from Router OS
     *
     * @param array       $response   Response data
     *
     * @return array                  Array with parsed data
     */
    public function parseResponse($response)
    {
        if (is_array($response)) {
            $PARSED      = array();
            $CURRENT     = null;
            $singlevalue = null;
            foreach ($response as $x) {
                if (in_array($x, array('!fatal','!re','!trap'))) {
                    if ($x == '!re') {
                        $CURRENT =& $PARSED[];
                    } else {
                        $CURRENT =& $PARSED[$x][];
                    }
                } elseif ($x != '!done') {
                    $MATCHES = array();
                    if (preg_match_all('/[^=]+/i', $x, $MATCHES)) {
                        if ($MATCHES[0][0] == 'ret') {
                            $singlevalue = $MATCHES[0][1];
                        }
                        $CURRENT[$MATCHES[0][0]] = (isset($MATCHES[0][1]) ? $MATCHES[0][1] : '');
                    }
                }
            }

            if (empty($PARSED) && !is_null($singlevalue)) {
                $PARSED = $singlevalue;
            }

            return $PARSED;
        } else {
            return array();
        }
    }


    /**
     * Parse response from Router OS
     *
     * @param array       $response   Response data
     *
     * @return array                  Array with parsed data
     */
    public function parseResponse4Smarty($response)
    {
        if (is_array($response)) {
            $PARSED      = array();
            $CURRENT     = null;
            $singlevalue = null;
            foreach ($response as $x) {
                if (in_array($x, array('!fatal','!re','!trap'))) {
                    if ($x == '!re') {
                        $CURRENT =& $PARSED[];
                    } else {
                        $CURRENT =& $PARSED[$x][];
                    }
                } elseif ($x != '!done') {
                    $MATCHES = array();
                    if (preg_match_all('/[^=]+/i', $x, $MATCHES)) {
                        if ($MATCHES[0][0] == 'ret') {
                            $singlevalue = $MATCHES[0][1];
                        }
                        $CURRENT[$MATCHES[0][0]] = (isset($MATCHES[0][1]) ? $MATCHES[0][1] : '');
                    }
                }
            }
            foreach ($PARSED as $key => $value) {
                $PARSED[$key] = $this->arrayChangeKeyName($value);
            }
            return $PARSED;
            if (empty($PARSED) && !is_null($singlevalue)) {
                $PARSED = $singlevalue;
            }
        } else {
            return array();
        }
    }


    /**
     * Change "-" and "/" from array key to "_"
     *
     * @param array       $array      Input array
     *
     * @return array                  Array with changed key names
     */
    public function arrayChangeKeyName(&$array)
    {
        if (is_array($array)) {
            foreach ($array as $k => $v) {
                $tmp = str_replace("-", "_", $k);
                $tmp = str_replace("/", "_", $tmp);
                if ($tmp) {
                    $array_new[$tmp] = $v;
                } else {
                    $array_new[$k] = $v;
                }
            }
            return $array_new;
        } else {
            return $array;
        }
    }


    /**
     * Read data from Router OS
     *
     * Flushes every queued sentence to the Go backend in one request and
     * returns the reply words as a flat array, exactly like the socket version
     * returned everything it read up to "!done".
     *
     * @param boolean     $parse      Parse the data? default: true
     *
     * @return array                  Array with parsed or unparsed data
     */
    public function read($parse = true)
    {
        $RESPONSE = array();

        $this->flushPending();

        if (!empty($this->sentences)) {
            $batch           = $this->sentences;
            $this->sentences = array();

            if ($this->session !== null) {
                $response = mikhmon_api_post('/v1/exec', array(
                    'session'    => $this->session,
                    'sentences'  => $batch,
                    'timeout_ms' => $this->timeoutMs(),
                ), mikhmon_api_exec_timeout());

                if (is_array($response) && !empty($response['ok']) && isset($response['results'])) {
                    foreach ($response['results'] as $result) {
                        if (empty($result['sentences'])) {
                            continue;
                        }
                        foreach ($result['sentences'] as $sentence) {
                            foreach ($sentence as $word) {
                                $RESPONSE[] = $word;
                            }
                        }
                        if (!empty($result['error'])) {
                            $this->error_str = $result['error'];
                            $this->debug('>>> ' . $result['error']);
                        }
                    }
                } else {
                    $this->error_str = mikhmon_api_error($response);
                    $this->debug('>>> ' . $this->error_str);
                }
            }
        }

        if ($parse) {
            $RESPONSE = $this->parseResponse($RESPONSE);
        }

        return $RESPONSE;
    }


    /**
     * Write (send) data to Router OS
     *
     * Sentences are buffered locally and shipped to the Go backend when the
     * command is terminated, which is exactly when the socket version would
     * have had a complete sentence on the wire.
     *
     * @param string      $command    A string with the command to send
     * @param mixed       $param2     If we set an integer, the command will send this data as a "tag"
     *                                If we set it to boolean true, the funcion will send the comand and finish
     *                                If we set it to boolean false, the funcion will send the comand and wait for next command
     *                                Default: true
     *
     * @return boolean                Return false if no command especified
     */
    public function write($command, $param2 = true)
    {
        if ($command) {
            $data = explode("\n", $command);
            foreach ($data as $com) {
                $com = trim($com);
                if ($com === '') {
                    continue;
                }
                $this->pending[] = $com;
                $this->debug('<<< [' . strlen($com) . '] ' . $com);
            }

            if (gettype($param2) == 'integer') {
                $this->pending[] = '.tag=' . $param2;
                $this->debug('<<< [' . strlen('.tag=' . $param2) . '] .tag=' . $param2);
                $this->pushSentence();
            } elseif (gettype($param2) == 'boolean') {
                if ($param2) {
                    $this->pushSentence();
                }
            }

            return true;
        } else {
            return false;
        }
    }


    /**
     * Write (send) data to Router OS
     *
     * Identical to the original implementation: it builds one sentence out of
     * the command and its parameters and then waits for the reply.
     *
     * @param string      $com        A string with the command to send
     * @param array       $arr        An array with arguments or queries
     *
     * @return array                  Array with parsed
     */
    public function comm($com, $arr = array())
    {
        $count = count($arr);
        $this->write($com, !$arr);
        $i = 0;
        if ($this->isIterable($arr)) {
            foreach ($arr as $k => $v) {
                switch ($k[0]) {
                    case "?":
                        $el = "$k=$v";
                        break;
                    case "~":
                        $el = "$k~$v";
                        break;
                    default:
                        $el = "=$k=$v";
                        break;
                }

                $last = ($i++ == $count - 1);
                $this->write($el, $last);
            }
        }

        return $this->read();
    }


    /**
     * Close the current sentence and queue it for the next read().
     *
     * @return void
     */
    private function pushSentence()
    {
        if (!empty($this->pending)) {
            $this->sentences[] = $this->pending;
            $this->pending     = array();
        }
    }

    /**
     * Queue whatever half-built sentence is left, so a script that writes
     * without reading does not silently drop the command.
     *
     * @return void
     */
    private function flushPending()
    {
        if (!empty($this->pending)) {
            $this->pushSentence();
        }
    }

    /**
     * Router timeout expressed in milliseconds for the Go backend.
     *
     * @return int
     */
    private function timeoutMs()
    {
        $ms = ((int) $this->timeout) * 1000;
        if ($ms < 1000) {
            $ms = 1000;
        }
        return $ms;
    }

    /**
     * Standard destructor
     *
     * @return void
     */
    public function __destruct()
    {
        $this->flushPending();
        $this->disconnect();
    }
}


/* -------------------------------------------------------------------------
 * Go backend transport
 *
 * The frontend talks to the mikhmon-api service over plain HTTP on the Docker
 * network. Configure it with:
 *
 *   MIKHMON_API_URL          base URL, default http://127.0.0.1:8088
 *   MIKHMON_API_TOKEN        optional shared secret
 *   MIKHMON_API_CONCURRENCY  worker count for bulk voucher creation
 * ---------------------------------------------------------------------- */

function mikhmon_api_base()
{
    static $base = null;
    if ($base === null) {
        $url = getenv('MIKHMON_API_URL');
        if ($url === false || trim($url) === '') {
            $url = 'http://127.0.0.1:8088';
        }
        $base = rtrim(trim($url), '/');
    }
    return $base;
}

function mikhmon_api_token()
{
    static $token = null;
    if ($token === null) {
        $t = getenv('MIKHMON_API_TOKEN');
        $token = ($t === false) ? '' : trim($t);
    }
    return $token;
}

/**
 * Seconds a connect probe may take. Mirrors the original retry budget
 * ($attempts * $delay) so "Not Connected" still appears after a comparable
 * wait, but a dead router no longer pins an FPM worker for 15+ seconds.
 */
function mikhmon_api_connect_timeout()
{
    static $secs = null;
    if ($secs === null) {
        $secs = (int) mikhmon_api_env('MIKHMON_API_CONNECT_TIMEOUT', 20);
        if ($secs < 2) {
            $secs = 2;
        }
    }
    return $secs;
}

/**
 * Seconds a normal command batch may take.
 */
function mikhmon_api_exec_timeout()
{
    static $secs = null;
    if ($secs === null) {
        $secs = (int) mikhmon_api_env('MIKHMON_API_EXEC_TIMEOUT', 120);
        if ($secs < 5) {
            $secs = 5;
        }
    }
    return $secs;
}

/**
 * Seconds a bulk voucher batch may take.
 */
function mikhmon_api_bulk_timeout()
{
    static $secs = null;
    if ($secs === null) {
        $secs = (int) mikhmon_api_env('MIKHMON_API_BULK_TIMEOUT', 600);
        if ($secs < 30) {
            $secs = 30;
        }
    }
    return $secs;
}

function mikhmon_api_env($key, $default)
{
    $v = getenv($key);
    if ($v === false || trim($v) === '') {
        return $default;
    }
    return trim($v);
}

/**
 * POST a JSON payload to the Go backend.
 *
 * A static cURL handle is kept so the connection to the backend stays open
 * between requests handled by the same FPM worker.
 *
 * @param string  $path        e.g. "/v1/exec"
 * @param array   $payload     JSON-encodable body
 * @param int     $timeoutSec  Total request timeout
 *
 * @return array|null          Decoded response, or null when unreachable
 */
function mikhmon_api_post($path, $payload, $timeoutSec = 120)
{
    $url  = mikhmon_api_base() . $path;
    $body = json_encode($payload);
    if ($body === false) {
        return null;
    }

    $headers = array(
        'Content-Type: application/json',
        'Accept: application/json',
    );
    $token = mikhmon_api_token();
    if ($token !== '') {
        $headers[] = 'X-Mikhmon-Token: ' . $token;
    }

    if (function_exists('curl_init')) {
        static $ch = null;
        if ($ch === null) {
            $ch = curl_init();
            curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
            curl_setopt($ch, CURLOPT_POST, true);
            curl_setopt($ch, CURLOPT_TCP_KEEPALIVE, 1);
            curl_setopt($ch, CURLOPT_CONNECTTIMEOUT, 5);
            curl_setopt($ch, CURLOPT_FOLLOWLOCATION, false);
        }

        curl_setopt($ch, CURLOPT_URL, $url);
        curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
        curl_setopt($ch, CURLOPT_POSTFIELDS, $body);
        curl_setopt($ch, CURLOPT_TIMEOUT, $timeoutSec);

        $out = curl_exec($ch);
        if ($out === false) {
            // Drop the handle so the next call reconnects cleanly.
            curl_close($ch);
            $ch = null;
            return null;
        }

        $decoded = json_decode($out, true);
        return is_array($decoded) ? $decoded : null;
    }

    $ctx = stream_context_create(array('http' => array(
        'method'        => 'POST',
        'header'        => implode("\r\n", $headers) . "\r\n",
        'content'       => $body,
        'timeout'       => $timeoutSec,
        'ignore_errors' => true,
    )));

    $out = @file_get_contents($url, false, $ctx);
    if ($out === false) {
        return null;
    }

    $decoded = json_decode($out, true);
    return is_array($decoded) ? $decoded : null;
}

/**
 * Human readable error for a backend response.
 */
function mikhmon_api_error($response)
{
    if (is_array($response) && !empty($response['error'])) {
        return $response['error'];
    }
    return 'mikhmon-api backend unreachable at ' . mikhmon_api_base();
}

/**
 * Create many hotspot users in parallel.
 *
 * Drop-in replacement for the sequential
 * `foreach ($users as $u) { $API->comm("/ip/hotspot/user/add", $u); }` loop.
 * The caller keeps building the usernames/passwords itself, so the generated
 * credentials stay byte-for-byte identical to the PHP implementation.
 *
 * @param RouterosAPI $API          A connected API instance
 * @param array       $users        List of param arrays for /ip/hotspot/user/add
 * @param int         $concurrency  Worker connections (0 = MIKHMON_API_CONCURRENCY)
 *
 * @return array  total / added / failed / errors
 */
function mikhmon_bulk_add_hotspot_users($API, $users, $concurrency = 0)
{
    $total = count($users);
    if ($total === 0) {
        return array('total' => 0, 'added' => 0, 'failed' => 0, 'errors' => array());
    }

    if (!is_object($API) || empty($API->session)) {
        return array(
            'total'  => $total,
            'added'  => 0,
            'failed' => $total,
            'errors' => array('not connected'),
        );
    }

    if ($concurrency < 1) {
        $concurrency = (int) mikhmon_api_env('MIKHMON_API_CONCURRENCY', 16);
    }

    $payload = array();
    foreach ($users as $u) {
        $payload[] = array(
            'name'              => isset($u['name']) ? $u['name'] : '',
            'password'          => isset($u['password']) ? $u['password'] : '',
            'server'            => isset($u['server']) ? $u['server'] : '',
            'profile'           => isset($u['profile']) ? $u['profile'] : '',
            'limit-uptime'      => isset($u['limit-uptime']) ? $u['limit-uptime'] : '',
            'limit-bytes-total' => isset($u['limit-bytes-total']) ? (int) $u['limit-bytes-total'] : 0,
            'comment'           => isset($u['comment']) ? $u['comment'] : '',
        );
    }

    $response = mikhmon_api_post('/v1/bulk/user-add', array(
        'session'     => $API->session,
        'users'       => $payload,
        'concurrency' => $concurrency,
        'timeout_ms'  => max(1000, ((int) $API->timeout) * 1000),
    ), mikhmon_api_bulk_timeout());

    if (!is_array($response) || empty($response['ok'])) {
        return array(
            'total'  => $total,
            'added'  => 0,
            'failed' => $total,
            'errors' => array(mikhmon_api_error($response)),
        );
    }

    return $response;
}


/**
 * Delete many RouterOS menu items in parallel.
 *
 * Drop-in replacement for a sequential remove loop, which costs one round trip
 * per item - clearing a 5000 voucher batch that way took minutes. This hands
 * the whole id list to the Go service, which works them across its connection
 * pool instead.
 *
 * @param RouterosAPI $API          A connected API instance
 * @param string      $command      e.g. "/ip/hotspot/user/remove"
 * @param array       $ids          List of .id values ("*1", "*A", ...)
 * @param int         $concurrency  Worker connections (0 = MIKHMON_API_CONCURRENCY)
 *
 * @return array  total / removed / failed / duration_ms / errors
 */
function mikhmon_bulk_remove_ids($API, $command, $ids, $concurrency = 0)
{
    $ids = array_values(array_filter((array) $ids, function ($id) {
        return $id !== "" && $id !== null;
    }));
    $total = count($ids);

    if ($total === 0) {
        return array('total' => 0, 'removed' => 0, 'failed' => 0, 'errors' => array());
    }

    if (!is_object($API) || empty($API->session)) {
        return array(
            'total'   => $total,
            'removed' => 0,
            'failed'  => $total,
            'errors'  => array('not connected'),
        );
    }

    if ($concurrency < 1) {
        $concurrency = (int) mikhmon_api_env('MIKHMON_API_CONCURRENCY', 16);
    }

    $response = mikhmon_api_post('/v1/bulk/remove', array(
        'session'     => $API->session,
        'command'     => $command,
        'ids'         => $ids,
        'concurrency' => $concurrency,
        'timeout_ms'  => max(1000, ((int) $API->timeout) * 1000),
    ), mikhmon_api_bulk_timeout());

    if (!is_array($response) || empty($response['ok'])) {
        return array(
            'total'   => $total,
            'removed' => 0,
            'failed'  => $total,
            'errors'  => array(mikhmon_api_error($response)),
        );
    }

    return $response;
}

/**
 * Delete many hotspot users in parallel.
 *
 * @param RouterosAPI $API          A connected API instance
 * @param array       $ids          List of hotspot user .id values
 * @param int         $concurrency  Worker connections (0 = MIKHMON_API_CONCURRENCY)
 *
 * @return array  total / removed / failed / duration_ms / errors
 */
function mikhmon_bulk_remove_hotspot_users($API, $ids, $concurrency = 0)
{
    return mikhmon_bulk_remove_ids($API, '/ip/hotspot/user/remove', $ids, $concurrency);
}
// encrypt decript

if (!function_exists('encrypt')) {
function encrypt($string, $key=128) {
	$result = '';
	for($i=0, $k= strlen($string); $i<$k; $i++) {
		$char = substr($string, $i, 1);
		$keychar = substr($key, ($i % strlen($key))-1, 1);
		$char = chr(ord($char)+ord($keychar));
		$result .= $char;
	}
	return base64_encode($result);
}
}

if (!function_exists('decrypt')) {
function decrypt($string, $key=128) {
	$result = '';
	$string = base64_decode($string);
	for($i=0, $k=strlen($string); $i< $k ; $i++) {
		$char = substr($string, $i, 1);
		$keychar = substr($key, ($i % strlen($key))-1, 1);
		$char = chr(ord($char)-ord($keychar));
		$result .= $char;
	}
	return $result;
}
}

// Reformat date time MikroTik
// by Laksamadi Guko

if (!function_exists('formatInterval')) {
function formatInterval($dtm){
$val_convert = $dtm;
$new_format = str_replace("s", "", str_replace("m", "m ", str_replace("h", "h ", str_replace("d", "d ", str_replace("w", "w ", $val_convert)))));
return $new_format;
}
}

if (!function_exists('formatDTM')) {
function formatDTM($dtm){
if(substr($dtm, 1,1) == "d" || substr($dtm, 2,1) == "d"){
    $day = explode("d",$dtm)[0]."d";
    $day = str_replace("d", "d ", str_replace("w", "w ", $day));
    $dtm = explode("d",$dtm)[1];
}elseif(substr($dtm, 1,1) == "w" && substr($dtm, 3,1) == "d" || substr($dtm, 2,1) == "w" && substr($dtm, 4,1) == "d"){
    $day = explode("d",$dtm)[0]."d";
    $day = str_replace("d", "d ", str_replace("w", "w ", $day));
    $dtm = explode("d",$dtm)[1];
}elseif (substr($dtm, 1,1) == "w" || substr($dtm, 2,1) == "w" ) {
    $day = explode("w",$dtm)[0]."w";
    $day = str_replace("d", "d ", str_replace("w", "w ", $day));
    $dtm = explode("w",$dtm)[1];
}

// secs
if(strlen($dtm) == "2" && substr($dtm, -1) == "s"){
    $format = $day." 00:00:0".substr($dtm, 0,-1);
}elseif(strlen($dtm) == "3" && substr($dtm, -1) == "s"){
    $format = $day." 00:00:".substr($dtm, 0,-1);
//minutes
}elseif(strlen($dtm) == "2" && substr($dtm, -1) == "m"){
    $format = $day." 00:0".substr($dtm, 0,-1).":00";
}elseif(strlen($dtm) == "3" && substr($dtm, -1) == "m"){
    $format = $day." 00:".substr($dtm, 0,-1).":00";
//hours
}elseif(strlen($dtm) == "2" && substr($dtm, -1) == "h"){
    $format = $day." 0".substr($dtm, 0,-1).":00:00";
}elseif(strlen($dtm) == "3" && substr($dtm, -1) == "h"){
    $format = $day." ".substr($dtm, 0,-1).":00:00";
 
//minutes -secs
}elseif(strlen($dtm) == "4" && substr($dtm, -1) == "s" && substr($dtm,1,-2) == "m"){
    $format = $day." "."00:0".substr($dtm, 0,1).":0".substr($dtm, 2,-1);
}elseif(strlen($dtm) == "5" && substr($dtm, -1) == "s" && substr($dtm,1,-3) == "m"){
    $format = $day." "."00:0".substr($dtm, 0,1).":".substr($dtm, 2,-1);
}elseif(strlen($dtm) == "5" && substr($dtm, -1) == "s" && substr($dtm,2,-2) == "m"){
    $format = $day." "."00:".substr($dtm, 0,2).":0".substr($dtm, 3,-1);
}elseif(strlen($dtm) == "6" && substr($dtm, -1) == "s" && substr($dtm,2,-3) == "m"){
    $format = $day." "."00:".substr($dtm, 0,2).":".substr($dtm, 3,-1);

//hours -secs
}elseif(strlen($dtm) == "4" && substr($dtm, -1) == "s" && substr($dtm,1,-2) == "h"){
    $format = $day." 0".substr($dtm, 0,1).":00:0".substr($dtm, 2,-1);
}elseif(strlen($dtm) == "5" && substr($dtm, -1) == "s" && substr($dtm,1,-3) == "h"){
    $format = $day." 0".substr($dtm, 0,1).":00:".substr($dtm, 2,-1);
}elseif(strlen($dtm) == "5" && substr($dtm, -1) == "s" && substr($dtm,2,-2) == "h"){
    $format = $day." ".substr($dtm, 0,2).":00:0".substr($dtm, 3,-1);
}elseif(strlen($dtm) == "6" && substr($dtm, -1) == "s" && substr($dtm,2,-3) == "h"){
    $format = $day." ".substr($dtm, 0,2).":00:".substr($dtm, 3,-1);

//hours -secs
}elseif(strlen($dtm) == "4" && substr($dtm, -1) == "m" && substr($dtm,1,-2) == "h"){
    $format = $day." 0".substr($dtm, 0,1).":0".substr($dtm, 2,-1).":00";
}elseif(strlen($dtm) == "5" && substr($dtm, -1) == "m" && substr($dtm,1,-3) == "h"){
    $format = $day." 0".substr($dtm, 0,1).":".substr($dtm, 2,-1).":00";
}elseif(strlen($dtm) == "5" && substr($dtm, -1) == "m" && substr($dtm,2,-2) == "h"){
    $format = $day." ".substr($dtm, 0,2).":0".substr($dtm, 3,-1).":00";
}elseif(strlen($dtm) == "6" && substr($dtm, -1) == "m" && substr($dtm,2,-3) == "h"){
    $format = $day." ".substr($dtm, 0,2).":".substr($dtm, 3,-1).":00";

//hours minutes secs
}elseif(strlen($dtm) == "6" && substr($dtm, -1) == "s" && substr($dtm,3,-2) == "m" && substr($dtm,1,-4) == "h"){
    $format = $day." 0".substr($dtm, 0,1).":0".substr($dtm, 2,-3).":0".substr($dtm, 4,-1);
}elseif(strlen($dtm) == "7" && substr($dtm, -1) == "s" && substr($dtm,3,-3) == "m" && substr($dtm,1,-5) == "h"){
    $format = $day." 0".substr($dtm, 0,1).":0".substr($dtm, 2,-4).":".substr($dtm, 4,-1);
}elseif(strlen($dtm) == "7" && substr($dtm, -1) == "s" && substr($dtm,4,-2) == "m" && substr($dtm,1,-5) == "h"){
    $format = $day." 0".substr($dtm, 0,1).":".substr($dtm, 2,-3).":0".substr($dtm, 5,-1);
}elseif(strlen($dtm) == "8" && substr($dtm, -1) == "s" && substr($dtm,4,-3) == "m" && substr($dtm,1,-6) == "h"){
    $format = $day." 0".substr($dtm, 0,1).":".substr($dtm, 2,-4).":".substr($dtm, 5,-1);
}elseif(strlen($dtm) == "7" && substr($dtm, -1) == "s" && substr($dtm,4,-2) == "m" && substr($dtm,2,-4) == "h"){
    $format = $day." ".substr($dtm, 0,2).":0".substr($dtm, 3,-3).":0".substr($dtm, 5,-1);
}elseif(strlen($dtm) == "8" && substr($dtm, -1) == "s" && substr($dtm,4,-3) == "m" && substr($dtm,2,-5) == "h"){
    $format = $day." ".substr($dtm, 0,2).":0".substr($dtm, 3,-4).":".substr($dtm, 5,-1);
}elseif(strlen($dtm) == "8" && substr($dtm, -1) == "s" && substr($dtm,5,-2) == "m" && substr($dtm,2,-5) == "h"){
    $format = $day." ".substr($dtm, 0,2).":".substr($dtm, 3,-3).":0".substr($dtm, 6,-1);
}elseif(strlen($dtm) == "9" && substr($dtm, -1) == "s" && substr($dtm,5,-3) == "m" && substr($dtm,2,-6) == "h"){
    $format = $day." ".substr($dtm, 0,2).":".substr($dtm, 3,-4).":".substr($dtm, 6,-1);

}else{
    $format = $dtm;
}
return $format;
}
}

if (!function_exists('randN')) {
function randN($length) {
	$chars = "23456789";
	$charArray = str_split($chars);
	$charCount = strlen($chars);
	$result = "";
	for($i=1;$i<=$length;$i++)
	{
		$randChar = rand(0,$charCount-1);
		$result .= $charArray[$randChar];
	}
	return $result;
}
}

if (!function_exists('randUC')) {
function randUC($length) {
	$chars = "ABCDEFGHJKLMNPRSTUVWXYZ";
	$charArray = str_split($chars);
	$charCount = strlen($chars);
	$result = "";
	for($i=1;$i<=$length;$i++)
	{
		$randChar = rand(0,$charCount-1);
		$result .= $charArray[$randChar];
	}
	return $result;
}
}

if (!function_exists('randLC')) {
function randLC($length) {
	$chars = "abcdefghijkmnprstuvwxyz";
	$charArray = str_split($chars);
	$charCount = strlen($chars);
	$result = "";
	for($i=1;$i<=$length;$i++)
	{
		$randChar = rand(0,$charCount-1);
		$result .= $charArray[$randChar];
	}
	return $result;
}
}

if (!function_exists('randULC')) {
function randULC($length) {
	$chars = "ABCDEFGHJKLMNPRSTUVWXYZabcdefghijkmnprstuvwxyz";
	$charArray = str_split($chars);
	$charCount = strlen($chars);
	$result = "";
	for($i=1;$i<=$length;$i++)
	{
		$randChar = rand(0,$charCount-1);
		$result .= $charArray[$randChar];
	}
	return $result;
}
}

if (!function_exists('randNLC')) {
function randNLC($length) {
	$chars = "23456789abcdefghijkmnprstuvwxyz";
	$charArray = str_split($chars);
	$charCount = strlen($chars);
	$result = "";
	for($i=1;$i<=$length;$i++)
	{
		$randChar = rand(0,$charCount-1);
		$result .= $charArray[$randChar];
	}
	return $result;
}
}

if (!function_exists('randNUC')) {
function randNUC($length) {
	$chars = "23456789ABCDEFGHJKLMNPRSTUVWXYZ";
	$charArray = str_split($chars);
	$charCount = strlen($chars);
	$result = "";
	for($i=1;$i<=$length;$i++)
	{
		$randChar = rand(0,$charCount-1);
		$result .= $charArray[$randChar];
	}
	return $result;
}
}

if (!function_exists('randNULC')) {
function randNULC($length) {
	$chars = "23456789ABCDEFGHJKLMNPRSTUVWXYZabcdefghijkmnprstuvwxyz";
	$charArray = str_split($chars);
	$charCount = strlen($chars);
	$result = "";
	for($i=1;$i<=$length;$i++)
	{
		$randChar = rand(0,$charCount-1);
		$result .= $charArray[$randChar];
	}
	return $result;
}
}

?>
