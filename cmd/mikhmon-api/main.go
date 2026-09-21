// Command mikhmon-api is the RouterOS backend for Mikhmon.
//
// The PHP frontend keeps serving the original Mikhmon UI; every RouterOS API
// call it used to make over a raw socket now goes through this service, which
// pools and reuses connections and can push a whole voucher batch in parallel.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Setiyoaryo/mikhmon-new/internal/api"
)

func main() {
	opts := api.Options{
		Addr:             env("MIKHMON_API_ADDR", ":8088"),
		Token:            env("MIKHMON_API_TOKEN", ""),
		MaxConnPerRouter: envInt("MIKHMON_API_MAX_CONN_PER_ROUTER", 32),
		IdleTimeout:      envDuration("MIKHMON_API_IDLE_TIMEOUT", 60*time.Second),
		SessionTTL:       envDuration("MIKHMON_API_SESSION_TTL", 10*time.Minute),
		ReadTimeout:      envDuration("MIKHMON_API_READ_TIMEOUT", 15*time.Second),
	}

	srv := api.New(opts)
	defer srv.Close()

	httpSrv := &http.Server{
		Addr:              opts.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// Voucher generation may legitimately take minutes for huge batches,
		// so only the header read and idle timeouts are enforced.
		IdleTimeout: 120 * time.Second,
		ErrorLog:    log.New(os.Stderr, "mikhmon-api: ", log.LstdFlags),
	}

	go func() {
		log.Printf("mikhmon-api listening on %s (max %d conn/router, idle %s, auth %s)",
			opts.Addr, opts.MaxConnPerRouter, opts.IdleTimeout, authLabel(opts.Token))

		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

func authLabel(token string) string {
	if token == "" {
		return "disabled"
	}
	return "bearer token"
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		log.Printf("invalid %s=%q, using %d", key, os.Getenv(key), def)
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
		log.Printf("invalid %s=%q, using %s", key, os.Getenv(key), def)
	}
	return def
}
