package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
)

func main() {
	// Initialize logger
	zlog := zerolog.New(os.Stdout).With().Timestamp().Logger()
	zlog.Info().Msg("Starting app-deployer orchestrator worker")

	// Create context that listens for interrupt signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// TODO: Initialize Redis connection
	// TODO: Initialize database connection
	// TODO: Start worker loop to process jobs from queue

	zlog.Info().Msg("Orchestrator worker started, waiting for jobs...")

	// Wait for interrupt signal
	<-ctx.Done()
	zlog.Info().Msg("Shutting down orchestrator worker gracefully...")

	// TODO: Cleanup and graceful shutdown
}
