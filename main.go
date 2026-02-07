// Package main is the entry point for the WhatsApp-style messaging server.
//
// LEARNING: In Go, the "main" package is special — it defines a standalone
// executable program. The main() function is where execution begins.
// Every Go executable must have exactly one main package with one main function.
//
// This file demonstrates several idiomatic Go patterns:
//   - Dependency injection via constructor functions (NewStore, NewService, etc.)
//   - Graceful shutdown using os/signal and context.Context
//   - Background goroutines managed by context cancellation
//   - The standard net/http server with custom timeouts
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
	// LEARNING: Go favors explicit dependency injection over global state.
	// We create concrete instances and pass them to constructors. This makes
	// the dependency graph clear and testing straightforward — you can swap
	// in mocks or fakes by passing different implementations.

	// Initialize store — the central in-memory data layer
	s := store.NewStore()

	// Initialize services — the business logic layer that operates on the store
	chatService := chat.NewService(s)
	attachmentService := attachment.NewService(s)

	// Initialize handlers — the HTTP/WebSocket layer that talks to clients
	chatHandler := chat.NewHandler(s, chatService)
	attachmentHandler := attachment.NewHandler(attachmentService)

	// LEARNING: http.NewServeMux() creates a request multiplexer (router).
	// It matches incoming request URLs to registered patterns and calls the
	// appropriate handler. This is Go's built-in router from the standard library.
	//
	// HandleFunc registers a function directly, while Handle registers a type
	// that implements the http.Handler interface (has a ServeHTTP method).
	// The attachmentHandler uses Handle because it implements http.Handler,
	// allowing it to route sub-paths internally.
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", chatHandler.HandleWebSocket)
	mux.Handle("/attachments", attachmentHandler)
	mux.Handle("/attachments/", attachmentHandler)

	// LEARNING: http.Server is a struct that configures the HTTP server.
	// Setting explicit timeouts is critical for production servers to prevent
	// resource exhaustion from slow or malicious clients:
	//   - ReadTimeout:  max time to read the entire request (headers + body)
	//   - WriteTimeout: max time to write the response
	//   - IdleTimeout:  max time to wait for the next request on a keep-alive connection
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// LEARNING: context.Context is Go's standard mechanism for managing
	// cancellation, deadlines, and request-scoped values across API boundaries.
	// context.WithCancel returns a derived context and a cancel function.
	// When cancel() is called, ctx.Done() channel closes, signaling all
	// goroutines listening on it to stop their work.
	//
	// context.Background() is the root context — it's never cancelled and has
	// no deadline. It's the starting point for all context trees.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// LEARNING: The TTL purger is a background worker that periodically cleans
	// up expired data. Running it with "go" launches it as a goroutine — a
	// lightweight concurrent function managed by Go's runtime scheduler.
	// Passing ctx allows us to cleanly stop the purger during shutdown.
	purger := store.NewTTLPurger(s, store.DefaultTTL, store.DefaultPurgeInterval)
	go purger.Start(ctx)

	// LEARNING: We start the HTTP server in a goroutine so the main goroutine
	// can continue to set up signal handling. ListenAndServe blocks until the
	// server stops, so without "go" we'd never reach the signal handling code.
	//
	// The error check "err != http.ErrServerClosed" distinguishes between a
	// graceful shutdown (expected) and an actual error (unexpected).
	// log.Fatalf logs and then calls os.Exit(1) — use it for unrecoverable errors.
	go func() {
		log.Printf("Server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// LEARNING: This is the idiomatic Go pattern for graceful shutdown.
	//
	// 1. Create a buffered channel (buffer size 1 so the signal sender doesn't block)
	// 2. Register it to receive SIGINT (Ctrl+C) and SIGTERM (Docker/K8s stop signal)
	// 3. Block on the channel with <-quit (receive operator) — the main goroutine
	//    sleeps here until a signal arrives
	//
	// Using a buffered channel with capacity 1 is important: signal.Notify
	// does not block when sending, so if the channel is unbuffered and nobody
	// is listening yet, the signal would be dropped.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Cancel the background context to stop all goroutines (like the TTL purger)
	cancel()

	// LEARNING: server.Shutdown gracefully drains active connections.
	// It stops accepting new connections and waits for in-flight requests
	// to complete before returning. The context timeout (30s) ensures we
	// don't wait forever if a connection is stuck.
	//
	// context.WithTimeout creates a context that auto-cancels after the
	// specified duration. Always defer the cancel function to release
	// resources even if the timeout isn't reached.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
