package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mikhmon/go-mikhmon/internal/config"
	"github.com/mikhmon/go-mikhmon/internal/web"
)

func main() {
	port := flag.Int("port", 8080, "HTTP port")
	configPath := flag.String("config", "", "Config file path (default: ~/.mikhmon/config.json)")
	flag.Parse()

	if *configPath == "" {
		home, _ := os.UserHomeDir()
		dir := filepath.Join(home, ".mikhmon")
		os.MkdirAll(dir, 0755)
		*configPath = filepath.Join(dir, "config.json")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal("config load: ", err)
	}

	srv := web.NewServer(cfg)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Mikhmon Go starting on http://0.0.0.0%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
