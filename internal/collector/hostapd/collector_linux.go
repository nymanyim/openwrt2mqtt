//go:build linux

package hostapd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/nymanyim/openwrt2mqtt/internal/collector"
)

// Collector follows hostapd log events emitted by OpenWrt logd.
type Collector struct {
	routerID string
	command  func(context.Context) (io.ReadCloser, func() error, error)
	resolve  func(string) map[string]any
}

func NewCollector(routerID string) *Collector {
	return &Collector{
		routerID: routerID,
		command:  logReader,
		resolve:  resolveLease,
	}
}

func (c *Collector) Name() string {
	return "hostapd"
}

func (c *Collector) Start(ctx context.Context, emitter collector.Emitter) error {
	if c.routerID == "" {
		return errors.New("hostapd router ID must not be empty")
	}
	if emitter == nil {
		return errors.New("hostapd emitter must not be nil")
	}
	output, wait, err := c.command(ctx)
	if err != nil {
		return fmt.Errorf("start logread: %w", err)
	}
	defer output.Close()

	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		observed := parseLine(c.routerID, scanner.Text(), time.Now(), c.resolve)
		if observed == nil {
			continue
		}
		if err := emitter.Emit(ctx, *observed); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("emit hostapd event: %w", err)
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read logread output: %w", err)
	}
	if err := wait(); err != nil {
		return fmt.Errorf("wait for logread: %w", err)
	}
	return errors.New("logread exited unexpectedly")
}

func logReader(ctx context.Context) (io.ReadCloser, func() error, error) {
	command := exec.CommandContext(ctx, "logread", "-f", "-l", "0", "-e", "hostapd")
	output, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := command.Start(); err != nil {
		return nil, nil, err
	}
	return output, command.Wait, nil
}

func interfaceName(line string) string {
	marker := "hostapd: "
	start := strings.Index(line, marker)
	if start < 0 {
		return ""
	}
	rest := line[start+len(marker):]
	end := strings.Index(rest, ":")
	if end <= 0 {
		return ""
	}
	return rest[:end]
}
