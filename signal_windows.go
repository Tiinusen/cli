//go:build windows

package cli

import (
	"context"
	"os"
	"os/signal"
)

func signalHandler(ctx context.Context, callback func()) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
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
