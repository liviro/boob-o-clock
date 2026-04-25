package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liviro/boob-o-clock/internal/api"
	"github.com/liviro/boob-o-clock/internal/store"
	"github.com/liviro/boob-o-clock/internal/web"
)

func main() {
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	dbPath := "boob-o-clock.db"
	cfg := api.Config{FerberEnabled: os.Getenv("FERBER_ENABLED") == "true"}
	flag.StringVar(&addr, "addr", addr, "listen address")
	flag.StringVar(&dbPath, "db", dbPath, "SQLite database path")
	flag.BoolVar(&cfg.FerberEnabled, "ferber", cfg.FerberEnabled, "enable Ferber sleep-training mode")
	flag.Parse()

	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer s.Close()

	handler := api.NewHandler(s, cfg)
	router := api.NewRouter(handler)

	// Serve embedded web files at root
	webContent, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		log.Fatalf("failed to create sub filesystem: %v", err)
	}
	fileServer := http.FileServer(http.FS(webContent))

	mux := http.NewServeMux()
	mux.Handle("/api/", router)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Ping(); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/", fileServer)

	srv := &http.Server{Addr: addr, Handler: mux}

	fmt.Printf("🕐 boob-o-clock listening on %s\n", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		log.Fatalf("server error: %v", err)
	case <-quit:
	}
	fmt.Println("\nshutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
}
