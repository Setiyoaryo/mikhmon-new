package main

import (
	"bytes"
	"sync/atomic"
	"testing"
	"time"
)

func TestLengthEncodingDecoding(t *testing.T) {
	testLengths := []int{
		0, 1, 50, 127,
		128, 500, 16383,
		16384, 100000, 2097151,
		2097152, 10000000, 268435455,
	}

	for _, l := range testLengths {
		enc := encodeLength(l)
		r := bytes.NewReader(enc)
		dec, err := decodeLength(r)
		if err != nil {
			t.Fatalf("length %d: decode error: %v", l, err)
		}
		if dec != l {
			t.Fatalf("length %d: decoded to %d", l, dec)
		}
	}
}

func newTestMockServer(t *testing.T, legacy bool) *MockRouterOSServer {
	s, err := NewMockServer("127.0.0.1:0", legacy)
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	return s
}

func TestModernLogin(t *testing.T) {
	s := newTestMockServer(t, false)
	defer s.Close()

	// 1. Successful login
	client, err := DialAndLogin(s.Addr, "admin", "secret", false, 5*time.Second)
	if err != nil {
		t.Fatalf("failed login: %v", err)
	}
	_ = client.Close()

	// 2. Failed login
	_, err = DialAndLogin(s.Addr, "admin", "wrong", false, 5*time.Second)
	if err == nil {
		t.Fatalf("expected login failure with wrong password")
	}
}

func TestLegacyLogin(t *testing.T) {
	s := newTestMockServer(t, true)
	defer s.Close()

	client, err := DialAndLogin(s.Addr, "admin", "secret", false, 5*time.Second)
	if err != nil {
		t.Fatalf("failed legacy login: %v", err)
	}
	_ = client.Close()
}

func TestRunPool5000Vouchers(t *testing.T) {
	s := newTestMockServer(t, false)
	defer s.Close()

	gen := NewGenerator()
	vouchers := gen.GenerateBatch(
		5000,
		"all",
		"vc",
		"VC-",
		"mix",
		"default",
		"2h",
		52428800,
		"batch-5000",
		5,
	)

	cfg := PoolConfig{
		Addr:        s.Addr,
		Username:    "admin",
		Password:    "secret",
		UseSSL:      false,
		Timeout:     10 * time.Second,
		Concurrency: 16,
		Mode:        "vc",
		CharType:    "mix",
		Prefix:      "VC-",
		UserLength:  5,
	}

	start := time.Now()
	res := RunPool(cfg, vouchers, gen)
	elapsed := time.Since(start)

	if !res.Success {
		t.Fatalf("RunPool failed: %s", res.Error)
	}
	if res.Count != 5000 {
		t.Fatalf("expected count 5000, got %d", res.Count)
	}

	totalReceived := atomic.LoadInt64(&s.UserCnt)
	if totalReceived != 5000 {
		t.Fatalf("server received %d users, expected 5000", totalReceived)
	}

	t.Logf("5000 vouchers created concurrently in %v (res duration: %d ms)", elapsed, res.DurationMs)
}

func TestDuplicateRetry(t *testing.T) {
	s := newTestMockServer(t, false)
	defer s.Close()

	gen := NewGenerator()
	// Force the first voucher to trigger duplicate once
	vouchers := []Voucher{
		{
			Name:      "force_dup_1",
			Password:  "force_dup_1",
			Server:    "all",
			Profile:   "default",
			TimeLimit: "1h",
			DataLimit: 0,
			Comment:   "test-dup",
		},
		{
			Name:      "normal_user",
			Password:  "normal_user",
			Server:    "all",
			Profile:   "default",
			TimeLimit: "1h",
			DataLimit: 0,
			Comment:   "test-dup",
		},
	}

	cfg := PoolConfig{
		Addr:        s.Addr,
		Username:    "admin",
		Password:    "secret",
		Concurrency: 2,
		Mode:        "vc",
		CharType:    "mix",
		Prefix:      "VC-",
		UserLength:  4,
	}

	res := RunPool(cfg, vouchers, gen)
	if !res.Success {
		t.Fatalf("RunPool failed: %s", res.Error)
	}
	if res.Count != 2 {
		t.Fatalf("expected count 2, got %d", res.Count)
	}
}
