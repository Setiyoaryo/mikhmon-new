package main

import (
	"crypto/rand"
	"fmt"
	"sync"
)

const (
	charsetN    = "23456789"
	charsetUC   = "ABCDEFGHJKLMNPRSTUVWXYZ"
	charsetLC   = "abcdefghijkmnprstuvwxyz"
	charsetULC  = "ABCDEFGHJKLMNPRSTUVWXYZabcdefghijkmnprstuvwxyz"
	charsetNLC  = "23456789abcdefghijkmnprstuvwxyz"
	charsetNUC  = "23456789ABCDEFGHJKLMNPRSTUVWXYZ"
	charsetNULC = "23456789ABCDEFGHJKLMNPRSTUVWXYZabcdefghijkmnprstuvwxyz"
)

// Voucher holds the configuration for a single Mikrotik hotspot user.
type Voucher struct {
	Name      string `json:"name"`
	Password  string `json:"password"`
	Server    string `json:"server"`
	Profile   string `json:"profile"`
	TimeLimit string `json:"timelimit"`
	DataLimit int64  `json:"datalimit"`
	Comment   string `json:"comment"`
}

// Generator produces unique hotspot user credentials matching Mikhmon's algorithms.
type Generator struct {
	mu   sync.Mutex
	seen map[string]bool
}

// NewGenerator creates a new Generator instance.
func NewGenerator() *Generator {
	return &Generator{
		seen: make(map[string]bool),
	}
}

// MarkUsed registers a username as already used.
func (g *Generator) MarkUsed(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seen[name] = true
}

// IsUsed checks whether a username is already taken.
func (g *Generator) IsUsed(name string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.seen[name]
}

// randomString picks n characters randomly from the given charset using crypto/rand with buffered sampling.
func randomString(charset string, length int) string {
	if length <= 0 {
		return ""
	}
	cLen := len(charset)
	if cLen == 0 {
		return ""
	}

	// Bitmask for power-of-2 rejection sampling
	mask := 1
	for mask < cLen {
		mask = (mask << 1) | 1
	}

	b := make([]byte, length)
	buf := make([]byte, length*2+8)

	filled := 0
	for filled < length {
		if _, err := rand.Read(buf); err != nil {
			panic(fmt.Sprintf("crypto/rand error: %v", err))
		}
		for _, rb := range buf {
			idx := int(rb) & mask
			if idx < cLen {
				b[filled] = charset[idx]
				filled++
				if filled == length {
					break
				}
			}
		}
	}
	return string(b)
}

// getCharset returns the charset matching Mikhmon's rand functions.
func getCharset(charType string) string {
	switch charType {
	case "lower":
		return charsetLC
	case "upper":
		return charsetUC
	case "upplow":
		return charsetULC
	case "mix":
		return charsetNLC
	case "mix1":
		return charsetNUC
	case "mix2":
		return charsetNULC
	case "num":
		return charsetN
	default:
		return charsetLC
	}
}

// aTable matches Mikhmon's $a array for userl offset:
// $a = array("1" => "", "", 1, 2, 2, 3, 3, 4);
var aTable = map[int]int{
	3: 1,
	4: 2,
	5: 2,
	6: 3,
	7: 3,
	8: 4,
}

// rawGenerateCredentials generates a single pair of username and password.
func rawGenerateCredentials(mode, charType, prefix string, userl int, extraChars int) (string, string) {
	if userl < 3 {
		userl = 3
	}
	if userl > 8 {
		userl = 8
	}

	effectiveLen := userl + extraChars

	if mode == "up" {
		// Username + Password mode
		cset := getCharset(charType)
		u := randomString(cset, effectiveLen)
		p := randomString(charsetN, userl)
		return prefix + u, p
	}

	// Default: "vc" mode (Username == Password)
	var username string
	switch charType {
	case "num":
		username = prefix + randomString(charsetN, effectiveLen)
	case "mix":
		username = prefix + randomString(charsetNLC, effectiveLen)
	case "mix1":
		username = prefix + randomString(charsetNUC, effectiveLen)
	case "mix2":
		username = prefix + randomString(charsetNULC, effectiveLen)
	default: // "lower", "upper", "upplow"
		aOffset, ok := aTable[userl]
		if !ok {
			aOffset = userl / 2
		}
		shuf := userl - aOffset
		if shuf < 1 {
			shuf = 1
		}
		cset := getCharset(charType)
		u := randomString(cset, shuf+extraChars)
		p := randomString(charsetN, aOffset)
		username = prefix + u + p
	}

	return username, username
}

// GenerateUniqueCredentials generates a unique username and password.
func (g *Generator) GenerateUniqueCredentials(mode, charType, prefix string, userl int) (string, string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	extraChars := 0
	attempts := 0

	for {
		u, p := rawGenerateCredentials(mode, charType, prefix, userl, extraChars)
		if !g.seen[u] {
			g.seen[u] = true
			return u, p
		}
		attempts++
		// If collision space is exhausted or getting saturated, expand length
		if attempts > 30 {
			extraChars++
			attempts = 0
		}
	}
}

// GenerateBatch produces a batch of vouchers with unique usernames.
func (g *Generator) GenerateBatch(qty int, server, mode, prefix, charType, profile, timelimit string, datalimit int64, comment string, userl int) []Voucher {
	vouchers := make([]Voucher, qty)
	for i := 0; i < qty; i++ {
		u, p := g.GenerateUniqueCredentials(mode, charType, prefix, userl)
		vouchers[i] = Voucher{
			Name:      u,
			Password:  p,
			Server:    server,
			Profile:   profile,
			TimeLimit: timelimit,
			DataLimit: datalimit,
			Comment:   comment,
		}
	}
	return vouchers
}
