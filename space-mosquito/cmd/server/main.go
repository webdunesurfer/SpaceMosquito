package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/vkh/spacemosquito/internal/app"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		<-sigChan
		cancel()
	}()

	if err := app.Run(ctx); err != nil {
		os.Exit(1)
	}
}
