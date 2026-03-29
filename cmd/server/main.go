package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/liviro/boob-o-clock/internal/api"
	"github.com/liviro/boob-o-clock/internal/store"
	"github.com/liviro/boob-o-clock/internal/web"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "boob-o-clock.db", "SQLite database path")
	flag.Parse()

	s, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer s.Close()

	handler := api.NewHandler(s)
	router := api.NewRouter(handler)

	// Serve embedded web files at root
	webContent, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		log.Fatalf("failed to create sub filesystem: %v", err)
	}
	fileServer := http.FileServer(http.FS(webContent))

	mux := http.NewServeMux()
	mux.Handle("/api/", router)
	mux.Handle("/", fileServer)

	fmt.Printf("🕐 boob-o-clock listening on %s\n", *addr)

	go func() {
		if err := http.ListenAndServe(*addr, mux); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("\nshutting down...")
}
