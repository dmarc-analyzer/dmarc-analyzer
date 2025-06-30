package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend/messageprocessor"
)

func main() {
	log.Println("Starting DMARC Analyzer SQS Consumer...")

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the message consumer in a goroutine
	go func() {
		if err := messageprocessor.StartMessageConsumer(ctx); err != nil {
			log.Printf("Message consumer error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down...", sig)

	// Cancel the context to stop the message consumer
	cancel()

	log.Println("DMARC Analyzer SQS Consumer stopped")
} 