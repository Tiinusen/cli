package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/ake-persson/mapslice-json"
)

// Must be created by Run(), handles the running cli application
type Runner struct {
	builder    *Builder
	ctx        context.Context
	cancelFunc context.CancelFunc
	flags      Flags
	args       Args
	isMain     bool
	config     interface{}
	flatConfig mapslice.MapSlice
}

// Application context
func (r *Runner) Context() context.Context {
	if r == nil {
		panic("runner is nil")
	}
	return r.ctx
}

// Returns true if no command or sub command has been invoked
func (r *Runner) IsMain() bool {
	return r.isMain
}

// Gracefully stops by cancelling application context
func (r *Runner) Stop() {
	if r == nil {
		return
	}
	r.cancelFunc()
}

// Forcefully exists application with os.Exit(exitCode)
func (r *Runner) Exit(errors ...error) {
	var hasErr bool
	for _, err := range errors {
		if err == nil {
			continue
		}
		hasErr = true
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	if hasErr {
		os.Exit(1)
	}
	os.Exit(0)
}

// Application global flags
func (r *Runner) Flags() Flags {
	if r == nil {
		return nil
	}
	return r.flags
}

// Application wide arguments
func (r *Runner) Args() Args {
	if r == nil {
		return nil
	}
	return r.args
}

// Returns the prased configuration that was set before .Run()
func (r *Runner) Config() interface{} {
	return r.config
}
