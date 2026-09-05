package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "argo-c",
	Short: "argo-c is a generic webhook reconciler",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Hello, Bounty Hunter!")

		// Set up signal handling for SIGINT and SIGTERM
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		var wg sync.WaitGroup

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
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
}
