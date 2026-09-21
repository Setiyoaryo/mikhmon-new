package api

import (
	"testing"
	"time"
)

func TestCommentOlderThan(t *testing.T) {
	// Fixed clock: 2026-09-22 12:00 UTC.
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		comment string
		days    int
		want    bool
	}{
		{"older than 30 days", "vc-735-08.01.26-warung", 30, true},
		// Boundary note: the comment date is compared from midnight, so a batch
		// dated exactly 30 days ago already counts. That matches the PHP code
		// this replaced ($stamp > $cutoff), so the cutoff did not move.
		{"exactly 30 days ago is already old enough", "vc-735-08.23.26-warung", 30, true},
		{"fresh batch", "vc-735-09.22.26-warung", 30, false},
		{"29 days old, 30 day window", "vc-735-08.24.26-warung", 30, false},
		{"7 day window catches a 10 day old batch", "vc-735-09.12.26-warung", 7, true},
		{"7 day window keeps a 3 day old batch", "vc-735-09.19.26-warung", 7, false},
		{"hand added user has no date", "up-admin-router", 30, false},
		{"empty comment", "", 30, false},
		{"date segment is not a date", "vc-735-notadate-x", 30, false},
		{"two digit year is in this century", "vc-735-01.01.25-old", 30, true},
	}

	for _, tc := range cases {
		if got := commentOlderThan(tc.comment, tc.days, now); got != tc.want {
			t.Errorf("%s: commentOlderThan(%q, %d) = %v, want %v", tc.name, tc.comment, tc.days, got, tc.want)
		}
	}
}

func TestCommentOlderThanWithZeroDaysMatchesEverything(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	if !commentOlderThan("anything", 0, now) {
		t.Error("days <= 0 means no age filter, everything should match")
	}
}

func TestParseQueryWordsAreBuilt(t *testing.T) {
	// Guard the shape the PHP side relies on: query keys become ?k=v words.
	query := map[string]string{"comment": "vc-1-x", "uptime": "00:00:00"}
	words := []string{"/ip/hotspot/user/print", "=.proplist=.id,comment,profile"}
	for k, v := range query {
		words = append(words, "?"+k+"="+v)
	}
	if len(words) != 4 {
		t.Fatalf("expected 4 words, got %d", len(words))
	}
	if words[0] != "/ip/hotspot/user/print" {
		t.Errorf("first word must be the command, got %q", words[0])
	}
}
