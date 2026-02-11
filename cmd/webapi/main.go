package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"wasa-text/service/api"
	"wasa-text/service/database"
	"wasa-text/service/globaltime"
)

func main() {

	cfg := loadConfiguration()

	db := database.NewInMemory(nil)
	clk := globaltime.NewSystemClock()

	a := api.New(api.Dependencies{
		DB:    db,
		Clock: clk,
	})

	apiHandler := a.Handler()

	webMux := http.NewServeMux()
	registerWebUI(webMux)

	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path

		// API routes (support both /api/* and legacy without /api)
		if strings.HasPrefix(p, "/api/") ||
			p == "/session" ||
			p == "/users" ||
			strings.HasPrefix(p, "/me/") ||
			strings.HasPrefix(p, "/conversations") ||
			strings.HasPrefix(p, "/messages") ||
			strings.HasPrefix(p, "/groups") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		// Everything else = Web UI / static
		webMux.ServeHTTP(w, r)
	}))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("WASAText backend running on http://" + cfg.HTTPAddr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = srv.Shutdown(ctx)
	log.Println("Bye.")
}
