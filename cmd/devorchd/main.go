package main

import (
	"fmt"
	"net/http"
	"os"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/log"
)

func main() {
	log.Init()

	cfg, err := config.Load()
	if err != nil {
		log.Errorf("config load failed: %v", err)
		os.Exit(1)
	}

	_, err = app.WireMinimal(cfg)
	if err != nil {
		log.Errorf("wire failed: %v", err)
		os.Exit(1)
	}

	// Step1 daemon: just health endpoint (later expand to SSE/gRPC)
	addr := cfg.DaemonAddr
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	log.Infof("devorchd listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
