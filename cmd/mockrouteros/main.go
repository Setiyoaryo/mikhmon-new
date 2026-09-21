// Command mockrouteros runs a fake RouterOS API endpoint for local testing.
//
//	go run ./cmd/mockrouteros -addr 127.0.0.1:8728 -user admin -pass secret
//
// It speaks the real RouterOS API wire protocol, so both the Go service and the
// PHP frontend can be pointed at it exactly as if it were a MikroTik device.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Setiyoaryo/mikhmon-new/internal/mockrouteros"
)

func main() {
	var (
		addr   = flag.String("addr", "127.0.0.1:8728", "address to listen on")
		user   = flag.String("user", "admin", "accepted username")
		pass   = flag.String("pass", "secret", "accepted password")
		legacy = flag.Bool("legacy", false, "use the pre-6.43 challenge/response login")
		quiet  = flag.Bool("quiet", false, "do not log every command")
	)
	flag.Parse()

	srv, err := mockrouteros.Start(*addr, mockrouteros.Options{
		Username: *user,
		Password: *pass,
		Legacy:   *legacy,
		Quiet:    *quiet,
	})
	if err != nil {
		log.Fatalf("mockrouteros: %v", err)
	}
	defer srv.Close()

	log.Printf("mockrouteros listening on %s (user=%s legacy=%t)", srv.Addr(), *user, *legacy)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}
