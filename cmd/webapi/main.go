package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	
	"wasa-text/service/api"
	"wasa-text/service/database"
	"wasa-text/service/globaltime"
)

func main() {
	
	cfg := loadConfiguration()

	
	db := database.NewInMemory()      
	clk := globaltime.NewSystemClock() 

	
	a := api.New(api.Dependencies{
		DB:    db,
		Clock: clk,
	})

	
	mux := http.NewServeMux()

	
	a.RegisterRoutes(mux)

	
	registerWebUI(mux)

	
	handler := withCORS(mux)

	
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
