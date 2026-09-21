package main

import (
	"strings"
	"testing"
	"time"
)

func TestGeneratorCharsets(t *testing.T) {
	modes := []string{"up", "vc"}
	charTypes := []string{"lower", "upper", "upplow", "mix", "mix1", "mix2", "num"}
	userls := []int{3, 4, 5, 6, 7, 8}

	for _, mode := range modes {
		for _, ct := range charTypes {
			for _, userl := range userls {
				gen := NewGenerator()
				u, p := gen.GenerateUniqueCredentials(mode, ct, "TST-", userl)

				if !strings.HasPrefix(u, "TST-") {
					t.Fatalf("expected prefix 'TST-', got '%s'", u)
				}
				trimmedU := strings.TrimPrefix(u, "TST-")
				if len(trimmedU) != userl {
					t.Fatalf("mode %s, char %s, userl %d: expected username len %d, got %d ('%s')",
						mode, ct, userl, userl, len(trimmedU), trimmedU)
				}

				if mode == "vc" {
					if u != p {
						t.Fatalf("in vc mode, expected username == password, got u='%s', p='%s'", u, p)
					}
				} else {
					// In up mode, password is randN (digits only, len userl)
					if len(p) != userl {
						t.Fatalf("in up mode, expected password len %d, got %d ('%s')", userl, len(p), p)
					}
					for _, ch := range p {
						if !strings.ContainsRune(charsetN, ch) {
							t.Fatalf("password '%s' contains invalid char '%c' for randN", p, ch)
						}
					}
				}
			}
		}
	}
}

func TestGenerate5000VouchersUniqueness(t *testing.T) {
	gen := NewGenerator()
	start := time.Now()
	vouchers := gen.GenerateBatch(
		5000,
		"all",
		"vc",
		"VCR-",
		"mix",
		"default",
		"1h",
		104857600,
		"vc-test-5000",
		6,
	)
	elapsed := time.Since(start)

	if len(vouchers) != 5000 {
		t.Fatalf("expected 5000 vouchers, got %d", len(vouchers))
	}

	seen := make(map[string]bool)
	for i, v := range vouchers {
		if !strings.HasPrefix(v.Name, "VCR-") {
			t.Fatalf("voucher %d has invalid prefix: %s", i, v.Name)
		}
		if seen[v.Name] {
			t.Fatalf("duplicate voucher name detected: %s at index %d", v.Name, i)
		}
		seen[v.Name] = true

		if v.Password != v.Name {
			t.Fatalf("voucher %d in vc mode has mismatched password: %s vs %s", i, v.Name, v.Password)
		}
		if v.Server != "all" || v.Profile != "default" || v.TimeLimit != "1h" || v.DataLimit != 104857600 || v.Comment != "vc-test-5000" {
			t.Fatalf("voucher %d has invalid attributes: %+v", i, v)
		}
	}

	t.Logf("Generated 5000 unique vouchers in %v", elapsed)
}

func TestExhaustedCombinationSpaceExpansion(t *testing.T) {
	// With userl=3 and char="num", max combinations is 8^3 = 512.
	// Generating 600 items should trigger length expansion and still produce 600 unique items without hanging.
	gen := NewGenerator()
	vouchers := gen.GenerateBatch(
		600,
		"all",
		"vc",
		"",
		"num",
		"default",
		"0",
		0,
		"test-expand",
		3,
	)

	if len(vouchers) != 600 {
		t.Fatalf("expected 600 vouchers, got %d", len(vouchers))
	}

	seen := make(map[string]bool)
	for i, v := range vouchers {
		if seen[v.Name] {
			t.Fatalf("duplicate detected at index %d: %s", i, v.Name)
		}
		seen[v.Name] = true
	}
}
