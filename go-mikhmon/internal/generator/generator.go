package generator

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/mikhmon/go-mikhmon/internal/routeros"
)

type UserMode string

const (
	ModeUserPass UserMode = "up"  // username + password
	ModeVoucher  UserMode = "vc"  // voucher (user=pass)
)

type CharSet string

const (
	CharLower    CharSet = "lower"
	CharUpper    CharSet = "upper"
	CharMixed    CharSet = "upplow"
	CharNumLower CharSet = "mix"
	CharNumUpper CharSet = "mix1"
	CharNumBoth  CharSet = "mix2"
	CharNum      CharSet = "num"
)

type GenerateRequest struct {
	Qty       int      `json:"qty"`
	Server    string   `json:"server"`
	Mode      UserMode `json:"mode"`
	Length    int      `json:"length"`
	Prefix    string   `json:"prefix"`
	Charset   CharSet  `json:"charset"`
	Profile   string   `json:"profile"`
	TimeLimit string   `json:"time_limit"`
	DataLimit int64    `json:"data_limit"` // bytes
	Comment   string   `json:"comment"`
}

type GeneratedUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Error    string `json:"error,omitempty"`
}

type GenerateResult struct {
	Users   []GeneratedUser `json:"users"`
	Comment string          `json:"comment"`
	Profile string          `json:"profile"`
	Elapsed string          `json:"elapsed"`
}

var charsets = map[CharSet]string{
	CharLower:    "abcdefghijkmnprstuvwxyz",
	CharUpper:    "ABCDEFGHJKLMNPRSTUVWXYZ",
	CharMixed:    "ABCDEFGHJKLMNPRSTUVWXYZabcdefghijkmnprstuvwxyz",
	CharNumLower: "23456789abcdefghijkmnprstuvwxyz",
	CharNumUpper: "23456789ABCDEFGHJKLMNPRSTUVWXYZ",
	CharNumBoth:  "23456789ABCDEFGHJKLMNPRSTUVWXYZabcdefghijkmnprstuvwxyz",
	CharNum:      "23456789",
}

func randString(cs CharSet, length int) string {
	chars := charsets[cs]
	if chars == "" {
		chars = charsets[CharNumLower]
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func randNum(length int) string {
	return randString(CharNum, length)
}

// Generate creates hotspot users concurrently on the MikroTik router.
// Uses a worker pool for maximum throughput.
func Generate(pool *routeros.Pool, req GenerateRequest, workers int) (*GenerateResult, error) {
	if workers <= 0 {
		workers = 10
	}
	if workers > 50 {
		workers = 50
	}

	start := time.Now()

	// Generate all usernames/passwords first (fast, in-memory)
	users := make([]GeneratedUser, req.Qty)
	comment := fmt.Sprintf("%s-%d-%s", req.Prefix, rand.Intn(900)+100, time.Now().Format("01.02.06"))
	if req.Comment != "" {
		comment += "-" + req.Comment
	}

	usedNames := make(map[string]bool, req.Qty)
	
	for i := 0; i < req.Qty; i++ {
		var username, password string
		for {
			switch req.Mode {
			case ModeUserPass:
				username = req.Prefix + randString(req.Charset, req.Length)
				password = randNum(req.Length)
			case ModeVoucher:
				if req.Charset == CharNum {
					code := randNum(req.Length)
					username = req.Prefix + code
				} else {
					code := randString(req.Charset, req.Length)
					username = req.Prefix + code
				}
				password = username
			default:
				username = req.Prefix + randString(CharNumLower, req.Length)
				password = username
			}
			// ensure unique
			if !usedNames[username] {
				usedNames[username] = true
				break
			}
		}
		users[i] = GeneratedUser{Username: username, Password: password}
	}

	// Create users on router concurrently using worker pool
	var wg sync.WaitGroup
	jobs := make(chan int, req.Qty)
	mu := &sync.Mutex{}
	errors := make([]string, req.Qty)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := pool.Get()
			if err != nil {
				// mark all remaining as error
				for idx := range jobs {
					mu.Lock()
					errors[idx] = err.Error()
					mu.Unlock()
				}
				return
			}
			defer pool.Put(client)

			for idx := range jobs {
				u := users[idx]
				args := map[string]string{
					"server":   req.Server,
					"name":     u.Username,
					"password": u.Password,
					"profile":  req.Profile,
					"comment":  comment,
				}
				if req.TimeLimit != "" && req.TimeLimit != "0" {
					args["limit-uptime"] = req.TimeLimit
				}
				if req.DataLimit > 0 {
					args["limit-bytes-total"] = fmt.Sprintf("%d", req.DataLimit)
				}
				_, err := client.RunArgs("/ip/hotspot/user/add", args)
				if err != nil {
					mu.Lock()
					errors[idx] = err.Error()
					mu.Unlock()
					// reconnect on error
					pool.Put(client)
					client, err = pool.Get()
					if err != nil {
						return
					}
				}
			}
		}()
	}

	for i := 0; i < req.Qty; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	// Collect errors
	for i, e := range errors {
		if e != "" {
			users[i].Error = e
		}
	}

	return &GenerateResult{
		Users:   users,
		Comment: comment,
		Profile: req.Profile,
		Elapsed: time.Since(start).String(),
	}, nil
}

// FormatBytes formats bytes to human readable string
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// FormatDTM converts MikroTik duration format to human readable
func FormatDTM(dtm string) string {
	if dtm == "" {
		return ""
	}
	// Replace MikroTik format separators
	dtm = strings.ReplaceAll(dtm, "w", "w ")
	dtm = strings.ReplaceAll(dtm, "d", "d ")
	dtm = strings.ReplaceAll(dtm, "h", "h ")
	dtm = strings.ReplaceAll(dtm, "m", "m ")
	dtm = strings.TrimSuffix(dtm, "s")
	return strings.TrimSpace(dtm)
}
