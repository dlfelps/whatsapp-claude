package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"whatsapp/attachment"
	"whatsapp/chat"
	"whatsapp/store"
)

func main() {
	// Initialize store
	s := store.NewStore()

	// Initialize services
	chatService := chat.NewService(s)
	attachmentService := attachment.NewService(s)

	// Initialize handlers
	chatHandler := chat.NewHandler(s, chatService)
	attachmentHandler := attachment.NewHandler(attachmentService)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", chatHandler.HandleWebSocket)
	mux.Handle("/attachments", attachmentHandler)
	mux.Handle("/attachments/", attachmentHandler)

	// Create server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start TTL purger
	purger := store.NewTTLPurger(s, store.DefaultTTL, store.DefaultPurgeInterval)
	go purger.Start(ctx)

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Cancel context to stop background goroutines
	cancel()

	// Shutdown server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
