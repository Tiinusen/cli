// Wraps urfave/cli and provides some ease of use functions allowing method chaining pattern
package cli

import (
	"fmt"
	"log"
	"time"

	"github.com/urfave/cli/v2"
)

func init() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Fprintf(c.App.Writer, "%s\n", c.App.Version)
	}
	log.SetFlags(0)
	log.SetOutput(&logWriter{})
}

type logWriter struct {
}

func (writer logWriter) Write(bytes []byte) (int, error) {
	return fmt.Print(time.Now().UTC().Format("2006-01-02 15:04:05") + ": " + string(bytes))
}

type Args []string

type Callback func(app *Runner, args Args, flags Flags) error

// BooleanFlag specifices a boolean flag variable input by provided name, usage and aliases
func BooleanFlag(name string, usage string, aliases ...string) cli.Flag {
	return &cli.BoolFlag{
		Name:    name,
		Usage:   usage,
		Aliases: aliases,
	}
}

// IntegerFlag specifices a integer flag variable input by provided name, usage and aliases
func IntegerFlag(name string, usage string, aliases ...string) cli.Flag {
	return &cli.IntFlag{
		Name:    name,
		Usage:   usage,
		Aliases: aliases,
	}
}

// StringFlag specifices a integer flag variable input by provided name, usage and aliases
func StringFlag(name string, usage string, aliases ...string) cli.Flag {
	return &cli.StringFlag{
		Name:    name,
		Usage:   usage,
		Aliases: aliases,
	}
}
