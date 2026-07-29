package app

import (
	"context"
	"fmt"
)

// App coordinates the openwrt2mqtt runtime.
type App struct {
	version string
}

func New(version string) *App {
	return &App{version: version}
}

func (a *App) Run(ctx context.Context) error {
	if a.version == "" {
		return fmt.Errorf("version must not be empty")
	}

	<-ctx.Done()
	return nil
}
