//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// Handles shutdown, interrupt and... signals
func signalHandler(ctx context.Context, callback func()) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	signal.Notify(signalChan, syscall.SIGINT)
	go func() {
		defer close(signalChan)
		defer signal.Stop(signalChan)
		defer callback()
		for {
			select {
			case <-signalChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}
