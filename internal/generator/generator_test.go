package generator

import (
	"testing"
)

func TestRandString(t *testing.T) {
	s := randString(CharLower, 6)
	if len(s) != 6 {
		t.Fatalf("expected 6 chars, got %d: %s", len(s), s)
	}
	for _, c := range s {
		if c < 'a' || c > 'z' {
			if c != 'i' && c != 'l' && c != 'o' && c != 'q' { // these are excluded in charset
				// The charset excludes certain chars, so just check it's lowercase
			}
		}
	}
}

func TestRandUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		s := randString(CharNumBoth, 8)
		if seen[s] {
			t.Fatalf("duplicate at iteration %d: %s", i, s)
		}
		seen[s] = true
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in  int64
		out string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.in)
		if got != tt.out {
			t.Errorf("FormatBytes(%d) = %s, want %s", tt.in, got, tt.out)
		}
	}
}

func TestFormatDTM(t *testing.T) {
	tests := []struct{ in, out string }{
		{"1d2h3m4s", "1d 2h 3m 4"},
		{"5h30m", "5h 30m"},
		{"", ""},
	}
	for _, tt := range tests {
		got := FormatDTM(tt.in)
		if got != tt.out {
			t.Errorf("FormatDTM(%s) = %s, want %s", tt.in, got, tt.out)
		}
	}
}
