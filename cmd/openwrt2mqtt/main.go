package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/nymanyim/openwrt2mqtt/internal/app"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.New(version).Run(ctx); err != nil {
		log.Fatal(err)
	}
}
