package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nymanyim/openwrt2mqtt/internal/app"
	"github.com/nymanyim/openwrt2mqtt/internal/config"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if handled, exitCode := handleCommand(ctx, os.Args[1:], os.Stdout, os.Stderr); handled {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return
	}

	runtimeConfig, err := config.FromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	runtimeApp, err := app.NewRuntime(ctx, version, runtimeConfig)
	if err != nil {
		log.Fatal(err)
	}
	if err := runtimeApp.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
