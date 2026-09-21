package generator

import (
	"regexp"
	"strings"
	"sync"
	"testing"
)

func TestBatchIsUniqueAndWellFormed(t *testing.T) {
	cases := []struct {
		mode     string
		charType string
		userl    int
		pattern  string
	}{
		{"vc", "lower", 4, `^[a-z]{2}[2-9]{2}$`},
		{"vc", "upper", 6, `^[A-Z]{3}[2-9]{3}$`},
		{"vc", "num", 5, `^[2-9]{5}$`},
		{"vc", "mix", 4, `^[a-z2-9]{4}$`},
		{"vc", "mix2", 4, `^[a-zA-Z2-9]{4}$`},
		{"up", "lower", 4, `^[a-z]{4}$`},
		{"up", "mix", 3, `^[a-z2-9]{3}$`},
	}

	for _, tc := range cases {
		g := New()
		got := g.Batch(BatchOptions{
			Qty:        200,
			Mode:       tc.mode,
			CharType:   tc.charType,
			UserLength: tc.userl,
		})
		if len(got) != 200 {
			t.Fatalf("%s/%s: expected 200 vouchers, got %d", tc.mode, tc.charType, len(got))
		}

		re := regexp.MustCompile(tc.pattern)
		seen := make(map[string]bool, len(got))
		for i, v := range got {
			if !re.MatchString(v.Name) {
				t.Fatalf("%s/%s: voucher %d name %q does not match %s", tc.mode, tc.charType, i, v.Name, tc.pattern)
			}
			if seen[v.Name] {
				t.Fatalf("%s/%s: duplicate name %q", tc.mode, tc.charType, v.Name)
			}
			seen[v.Name] = true

			if tc.mode == "vc" && v.Password != v.Name {
				t.Fatalf("vc mode must use the username as password, got %q/%q", v.Name, v.Password)
			}
			if tc.mode == "up" {
				if !regexp.MustCompile(`^[2-9]{` + string(rune('0'+tc.userl)) + `}$`).MatchString(v.Password) {
					t.Fatalf("up mode password %q must be %d digits", v.Password, tc.userl)
				}
			}
		}
	}
}

func TestPrefixIsApplied(t *testing.T) {
	g := New()
	for _, v := range g.Batch(BatchOptions{Qty: 50, Mode: "vc", CharType: "num", UserLength: 4, Prefix: "WIFI"}) {
		if !strings.HasPrefix(v.Name, "WIFI") {
			t.Fatalf("expected prefix WIFI, got %q", v.Name)
		}
	}
}

func TestVoucherFieldsAreCopied(t *testing.T) {
	g := New()
	got := g.Batch(BatchOptions{
		Qty:        3,
		Server:     "hotspot1",
		Mode:       "vc",
		CharType:   "num",
		Profile:    "1day",
		TimeLimit:  "1d",
		DataLimit:  1048576,
		Comment:    "batch-1",
		UserLength: 5,
	})
	for _, v := range got {
		if v.Server != "hotspot1" || v.Profile != "1day" || v.TimeLimit != "1d" ||
			v.DataLimit != 1048576 || v.Comment != "batch-1" {
			t.Fatalf("fields not copied: %+v", v)
		}
	}
}

// A small space (3 chars, digits only) must still yield unique names by
// widening instead of silently repeating: the router would reject duplicates.
func TestSaturatedSpaceStillUnique(t *testing.T) {
	g := New()
	got := g.Batch(BatchOptions{Qty: 2000, Mode: "vc", CharType: "num", UserLength: 3})
	seen := make(map[string]bool, len(got))
	for _, v := range got {
		if seen[v.Name] {
			t.Fatalf("duplicate name under saturation: %q", v.Name)
		}
		seen[v.Name] = true
	}
}

func TestConcurrentCredentialsAreUnique(t *testing.T) {
	g := New()
	const workers = 8
	const each = 200

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = make(map[string]bool, workers*each)
		dups []string
	)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				u, _ := g.Credentials("vc", "mix", "", 5)
				mu.Lock()
				if seen[u] {
					dups = append(dups, u)
				}
				seen[u] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(dups) > 0 {
		t.Fatalf("generator handed out %d duplicates, e.g. %v", len(dups), dups[:1])
	}
}

func TestMarkUsedAvoidsExistingNames(t *testing.T) {
	g := New()
	g.MarkUsed("AAAA")
	if !g.IsUsed("AAAA") {
		t.Fatal("MarkUsed did not register the name")
	}
	for _, v := range g.Batch(BatchOptions{Qty: 100, Mode: "vc", CharType: "num", UserLength: 4}) {
		if v.Name == "AAAA" {
			t.Fatal("generator produced a name marked as used")
		}
	}
}

func TestValidators(t *testing.T) {
	if ParseMode("up") != "up" || ParseMode("vc") != "vc" || ParseMode("junk") != "vc" {
		t.Fatal("ParseMode is wrong")
	}
	if ValidateCharType("num") != "num" || ValidateCharType("junk") != "mix" {
		t.Fatal("ValidateCharType is wrong")
	}
	if ClampUserLength(1) != 3 || ClampUserLength(4) != 4 || ClampUserLength(99) != 8 {
		t.Fatal("ClampUserLength is wrong")
	}
}
