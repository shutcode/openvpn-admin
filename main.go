package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shutcode/openvpn-admin/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// For now, use simple setup until full dependency injection is set up
	mux := http.NewServeMux()

	// API routes - temporarily using legacy handlers until full service integration
	setupLegacyRoutes(mux)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Static files - dashboard
	if _, err := os.Stat("./dashboard"); err == nil {
		fs := http.FileServer(http.Dir("./dashboard"))
		mux.Handle("/", fs)
	}

	// Apply middleware using new API package middleware
	handler := api.Chain(
		api.RecoveryMiddleware,
		api.LoggingMiddleware,
		api.CORSMiddleware([]string{"*"}),
	)(mux)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down gracefully...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("OpenVPN Dashboard starting on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// setupLegacyRoutes sets up the old-style handlers for backward compatibility
// These will be replaced when full dependency injection is implemented
func setupLegacyRoutes(mux *http.ServeMux) {
	// These are placeholders - the actual handlers would need the service dependencies
	// For now, return 503 Service Unavailable for API routes
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"success":false,"error":"API not yet initialized - full service dependencies required"}`))
	})
}
