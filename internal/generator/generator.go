// Package generator reproduces Mikhmon's voucher credential algorithms exactly,
// so generated usernames/passwords look the same as the PHP version's.
package generator

import (
	"crypto/rand"
	"sync"
	"sync/atomic"
)

// Character sets used by the original randN/randLC/randUC/randULC/randNLC/
// randNUC/randNULC helpers in lib/routeros_api.class.php.
const (
	charsetN    = "23456789"
	charsetUC   = "ABCDEFGHJKLMNPRSTUVWXYZ"
	charsetLC   = "abcdefghijkmnprstuvwxyz"
	charsetULC  = "ABCDEFGHJKLMNPRSTUVWXYZabcdefghijkmnprstuvwxyz"
	charsetNLC  = "23456789abcdefghijkmnprstuvwxyz"
	charsetNUC  = "23456789ABCDEFGHJKLMNPRSTUVWXYZ"
	charsetNULC = "23456789ABCDEFGHJKLMNPRSTUVWXYZabcdefghijkmnprstuvwxyz"
)

// aTable mirrors Mikhmon's `$a = array("1" => "", "", 1, 2, 2, 3, 3, 4);`
// which decides how many random digits are appended in "vc" mode.
var aTable = map[int]int{3: 1, 4: 2, 5: 2, 6: 3, 7: 3, 8: 4}

// Voucher is one MikroTik hotspot user to create.
type Voucher struct {
	Name      string `json:"name"`
	Password  string `json:"password"`
	Server    string `json:"server"`
	Profile   string `json:"profile"`
	TimeLimit string `json:"limit-uptime"`
	DataLimit int64  `json:"limit-bytes-total"`
	Comment   string `json:"comment"`
}

// Generator hands out unique credentials.
type Generator struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// New creates an empty Generator.
func New() *Generator {
	return &Generator{seen: make(map[string]struct{})}
}

// MarkUsed registers a username as taken (e.g. one already on the router).
func (g *Generator) MarkUsed(name string) {
	g.mu.Lock()
	g.seen[name] = struct{}{}
	g.mu.Unlock()
}

// IsUsed reports whether a username has already been handed out.
func (g *Generator) IsUsed(name string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.seen[name]
	return ok
}

var fallbackCounter uint64

// randomString picks n characters uniformly from charset using crypto/rand and
// rejection sampling (no modulo bias, unlike PHP's rand()).
func randomString(charset string, length int) string {
	if length <= 0 || charset == "" {
		return ""
	}
	cLen := len(charset)
	mask := 1
	for mask < cLen {
		mask = (mask << 1) | 1
	}

	out := make([]byte, length)
	buf := make([]byte, length*2+16)

	for filled := 0; filled < length; {
		if _, err := rand.Read(buf); err != nil {
			// crypto/rand should never fail; degrade instead of panicking in a
			// web request.
			idx := int(atomic.AddUint64(&fallbackCounter, 1)) % cLen
			out[filled] = charset[idx]
			filled++
			continue
		}
		for _, r := range buf {
			idx := int(r) & mask
			if idx < cLen {
				out[filled] = charset[idx]
				filled++
				if filled == length {
					break
				}
			}
		}
	}
	return string(out)
}

func charsetFor(charType string) string {
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

// rawCredentials mirrors the PHP generation branches in hotspot/generateuser.php.
//
//	mode "up": username = prefix + charset(userl), password = randN(userl)
//	mode "vc": username == password
//	   - lower/upper/upplow: prefix + letters(userl-offset) + digits(offset)
//	   - num/mix/mix1/mix2:  prefix + charset(userl)
//
// extraChars widens the random part only when the batch saturates the space.
func rawCredentials(mode, charType, prefix string, userl, extraChars int) (string, string) {
	if userl < 3 {
		userl = 3
	}
	if userl > 8 {
		userl = 8
	}
	width := userl + extraChars

	if mode == "up" {
		u := prefix + randomString(charsetFor(charType), width)
		p := randomString(charsetN, width) // PHP: randN($userl)
		return u, p
	}

	switch charType {
	case "num", "mix", "mix1", "mix2":
		u := prefix + randomString(charsetFor(charType), width)
		return u, u
	default:
		offset, ok := aTable[userl]
		if !ok {
			offset = userl / 2
		}
		shuf := userl - offset
		if shuf < 1 {
			shuf = 1
		}
		user := prefix + randomString(charsetFor(charType), shuf+extraChars) + randomString(charsetN, offset)
		return user, user
	}
}

// Credentials returns a unique (username, password) pair. If the configured
// length cannot hold the requested quantity the random part grows by one
// character rather than silently emitting duplicates the router would reject.
func (g *Generator) Credentials(mode, charType, prefix string, userl int) (string, string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	extra := 0
	for attempts := 0; ; attempts++ {
		u, p := rawCredentials(mode, charType, prefix, userl, extra)
		if _, used := g.seen[u]; !used {
			g.seen[u] = struct{}{}
			return u, p
		}
		if attempts > 30 {
			extra++
			attempts = 0
		}
	}
}

// BatchOptions describes one generation run.
type BatchOptions struct {
	Qty        int
	Server     string
	Mode       string
	Prefix     string
	CharType   string
	Profile    string
	TimeLimit  string
	DataLimit  int64
	Comment    string
	UserLength int
}

// Batch produces qty vouchers with unique usernames.
func (g *Generator) Batch(o BatchOptions) []Voucher {
	if o.Qty < 1 {
		o.Qty = 1
	}
	vouchers := make([]Voucher, o.Qty)
	for i := 0; i < o.Qty; i++ {
		u, p := g.Credentials(o.Mode, o.CharType, o.Prefix, o.UserLength)
		vouchers[i] = Voucher{
			Name:      u,
			Password:  p,
			Server:    o.Server,
			Profile:   o.Profile,
			TimeLimit: o.TimeLimit,
			DataLimit: o.DataLimit,
			Comment:   o.Comment,
		}
	}
	return vouchers
}

// ParseMode normalises the mode value coming from the PHP form.
func ParseMode(m string) string {
	if m == "up" {
		return "up"
	}
	return "vc"
}

// ValidateCharType clamps the charset selector to a known value.
func ValidateCharType(c string) string {
	switch c {
	case "lower", "upper", "upplow", "mix", "mix1", "mix2", "num":
		return c
	}
	return "mix"
}

// ClampUserLength keeps the length within the range the UI offers.
func ClampUserLength(n int) int {
	if n < 3 {
		return 3
	}
	if n > 8 {
		return 8
	}
	return n
}
