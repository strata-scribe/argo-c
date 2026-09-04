package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	fmt.Println("Hello, Bounty Hunter!")

	// Set up signal handling for SIGINT and SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	// Example of simulating a webhook reconciliation
	// In a real app, this would wrap the actual reconciliation logic.
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	// Reconcile logic here
	// }()

	// Wait for termination signal
	<-ctx.Done()
	log.Println("Received shutdown signal, initiating grace period...")

	// Create a timeout context for the grace period
	graceCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Use a channel to wait for WaitGroup
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All in-flight reconciliations finished successfully.")
	case <-graceCtx.Done():
		log.Println("Grace period timed out. Some reconciliations may not have finished.")
	}

	log.Println("Exiting.")
}
