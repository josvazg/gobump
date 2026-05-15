package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/josvazg/gobump/internal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	os.Exit(internal.Run(ctx, os.Args[1:], os.Getenv))
}
