package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// RequestConfig represents the configuration payload.
type RequestConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Pass        string `json:"pass"`
	SSL         bool   `json:"ssl"`
	Qty         int    `json:"qty"`
	Server      string `json:"server"`
	Mode        string `json:"mode"`
	UserLength  int    `json:"userl"`
	Prefix      string `json:"prefix"`
	CharType    string `json:"char"`
	Profile     string `json:"profile"`
	TimeLimit   string `json:"timelimit"`
	DataLimit   int64  `json:"datalimit"`
	Comment     string `json:"comment"`
	Concurrency int    `json:"concurrency"`
	TimeoutSec  int    `json:"timeout"`
}

func main() {
	var (
		flagHost        = flag.String("host", "", "RouterOS host or IP")
		flagPort        = flag.Int("port", 8728, "RouterOS API port")
		flagUser        = flag.String("user", "", "RouterOS API username")
		flagPass        = flag.String("pass", "", "RouterOS API password")
		flagSSL         = flag.Bool("ssl", false, "Use SSL/TLS")
		flagQty         = flag.Int("qty", 1, "Number of vouchers to generate (1 to 5000+)")
		flagServer      = flag.String("server", "all", "Hotspot server name")
		flagMode        = flag.String("mode", "vc", "User mode: 'up' (user & password) or 'vc' (user == password)")
		flagUserLength  = flag.Int("userl", 4, "Length of username/password (3 to 8)")
		flagPrefix      = flag.String("prefix", "", "Prefix for username")
		flagCharType    = flag.String("char", "mix", "Character set: lower, upper, upplow, mix, mix1, mix2, num")
		flagProfile     = flag.String("profile", "default", "Hotspot user profile")
		flagTimeLimit   = flag.String("timelimit", "0", "Limit uptime (e.g. 1h or 0)")
		flagDataLimit   = flag.Int64("datalimit", 0, "Limit bytes total")
		flagComment     = flag.String("comment", "", "Comment to tag generated users")
		flagConcurrency = flag.Int("concurrency", 16, "Number of concurrent worker connections")
		flagTimeoutSec  = flag.Int("timeout", 10, "Socket timeout in seconds")
		flagStdin       = flag.Bool("stdin", false, "Read configuration as JSON from STDIN")
		flagMockServer  = flag.String("mock-server", "", "Run mock RouterOS API server on address (e.g. 127.0.0.1:18728)")
	)
	flag.Parse()

	if *flagMockServer != "" {
		RunMockServerCLI(*flagMockServer)
		return
	}

	var req RequestConfig

	// Determine whether to read from STDIN
	readStdin := *flagStdin
	if !readStdin {
		// Check if STDIN is piped
		stat, err := os.Stdin.Stat()
		if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			readStdin = true
		}
	}

	if readStdin {
		data, err := io.ReadAll(os.Stdin)
		if err == nil && len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &req); err != nil {
				outputJSON(PoolResult{
					Success: false,
					Error:   fmt.Sprintf("invalid JSON input: %v", err),
				})
				os.Exit(1)
			}
		}
	}

	// Override with CLI flags if provided
	if req.Host == "" && *flagHost != "" {
		req.Host = *flagHost
	}
	if req.Port == 0 && *flagPort != 0 {
		req.Port = *flagPort
	}
	if req.User == "" && *flagUser != "" {
		req.User = *flagUser
	}
	if req.Pass == "" && *flagPass != "" {
		req.Pass = *flagPass
	}
	if !req.SSL && *flagSSL {
		req.SSL = *flagSSL
	}
	if req.Qty == 0 && *flagQty != 0 {
		req.Qty = *flagQty
	}
	if req.Server == "" {
		req.Server = *flagServer
	}
	if req.Mode == "" {
		req.Mode = *flagMode
	}
	if req.UserLength == 0 {
		req.UserLength = *flagUserLength
	}
	if req.Prefix == "" && *flagPrefix != "" {
		req.Prefix = *flagPrefix
	}
	if req.CharType == "" {
		req.CharType = *flagCharType
	}
	if req.Profile == "" {
		req.Profile = *flagProfile
	}
	if req.TimeLimit == "" {
		req.TimeLimit = *flagTimeLimit
	}
	if req.DataLimit == 0 && *flagDataLimit != 0 {
		req.DataLimit = *flagDataLimit
	}
	if req.Comment == "" && *flagComment != "" {
		req.Comment = *flagComment
	}
	if req.Concurrency == 0 {
		req.Concurrency = *flagConcurrency
	}
	if req.TimeoutSec == 0 {
		req.TimeoutSec = *flagTimeoutSec
	}

	// Sanitize and validate inputs
	if req.Host == "" {
		outputJSON(PoolResult{
			Success: false,
			Error:   "host is required",
		})
		os.Exit(1)
	}

	// Parse host:port if port is in host string
	host := req.Host
	port := req.Port
	if strings.Contains(host, ":") {
		h, p, err := net.SplitHostPort(host)
		if err == nil {
			host = h
			parsedPort, err := strconv.Atoi(p)
			if err == nil && parsedPort > 0 {
				port = parsedPort
			}
		}
	}
	if port <= 0 {
		if req.SSL {
			port = 8729
		} else {
			port = 8728
		}
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	if req.Qty < 1 {
		req.Qty = 1
	}
	if req.Qty > 10000 {
		req.Qty = 10000
	}
	if req.UserLength < 3 {
		req.UserLength = 3
	}
	if req.UserLength > 8 {
		req.UserLength = 8
	}
	if req.Mode != "up" && req.Mode != "vc" {
		req.Mode = "vc"
	}
	if req.Concurrency <= 0 {
		req.Concurrency = 16
	}
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = 10
	}

	// Generate batch of unique vouchers
	gen := NewGenerator()
	vouchers := gen.GenerateBatch(
		req.Qty,
		req.Server,
		req.Mode,
		req.Prefix,
		req.CharType,
		req.Profile,
		req.TimeLimit,
		req.DataLimit,
		req.Comment,
		req.UserLength,
	)

	poolCfg := PoolConfig{
		Addr:        addr,
		Username:    req.User,
		Password:    req.Pass,
		UseSSL:      req.SSL,
		Timeout:     time.Duration(req.TimeoutSec) * time.Second,
		Concurrency: req.Concurrency,
		Mode:        req.Mode,
		CharType:    req.CharType,
		Prefix:      req.Prefix,
		UserLength:  req.UserLength,
	}

	result := RunPool(poolCfg, vouchers, gen)
	outputJSON(result)

	if !result.Success {
		os.Exit(1)
	}
}

func outputJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
