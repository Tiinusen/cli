package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/ake-persson/mapslice-json"
	cli "github.com/urfave/cli/v2"
)

func New(name, usage string, version ...string) *Builder {
	var v string
	if len(version) > 0 {
		v = version[0]
	}
	return &Builder{
		app: &cli.App{
			Name:    name,
			Usage:   usage,
			Version: v,
		},
	}
}

// Must be created by New(), handles building the cli application
type Builder struct {
	preventMain     bool
	daemoize        bool
	before          Callback
	app             *cli.App
	runner          *Runner
	config          interface{}
	configStructure mapslice.MapSlice
}

// Parses args and runs cli application
func (b *Builder) Run(args ...string) (*Runner, error) {
	if b.runner != nil {
		return nil, fmt.Errorf(".Run() has already been called once")
	}

	b.runner = &Runner{
		builder: b,
		flags:   make(Flags),
		isMain:  true,
	}
	b.runner.ctx, b.runner.cancelFunc = context.WithCancel(context.Background())
	signalHandler(b.runner.ctx, b.runner.cancelFunc)

	b.preConfig()

	b.app.Before = func(c *cli.Context) error {
		for _, flagName := range c.LocalFlagNames() {
			b.runner.flags[flagName] = c.Value(flagName)
		}

		b.runner.args = Args(c.Args().Slice())

		b.postConfig(c)
		if b.before != nil {
			b.before(b.runner, b.runner.Args(), b.runner.Flags())
		}
		return nil
	}

	if !b.preventMain {
		b.app.Action = func(c *cli.Context) error {
			return nil
		}
	}

	helpFlagUsed := false // To prevent --help making app proceed
	for _, arg := range os.Args {
		if arg == "-h" || arg == "--help" {
			helpFlagUsed = true
		}
	}

	versionFlagUsed := false // To prevent --version making app proceed
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--version" {
			versionFlagUsed = true
		}
	}

	err := b.app.Run(os.Args)
	if b.preventMain {
		b.runner.Exit(err)
	} else if helpFlagUsed || versionFlagUsed {
		b.runner.Exit(nil)
	}

	if err != nil {
		return b.runner, err
	}
	return b.runner, nil
}

// BooleanFlag specifices a boolean flag variable input by provided name, usage and aliases
func (b *Builder) BooleanFlag(name string, usage string, aliases ...string) *Builder {
	b.app.Flags = append(b.app.Flags, BooleanFlag(name, usage, aliases...))
	return b
}

// IntegerFlag specifices a integer flag variable input by provided name, usage and aliases
func (b *Builder) IntegerFlag(name string, usage string, aliases ...string) *Builder {
	b.app.Flags = append(b.app.Flags, IntegerFlag(name, usage, aliases...))
	return b
}

// StringFlag specifices a integer flag variable input by provided name, usage and aliases
func (b *Builder) StringFlag(name string, usage string, aliases ...string) *Builder {
	b.app.Flags = append(b.app.Flags, StringFlag(name, usage, aliases...))
	return b
}

// Makes application exit after Run() finishes instead of a normal return
func (b *Builder) DisableMain() *Builder {
	b.preventMain = true
	return b
}

// Sets callback to be invoked before any command is executed
func (b *Builder) Before(callback Callback) *Builder {
	b.before = callback
	return b
}

func (b *Builder) Command(name string, usage string, callback Callback, flags ...cli.Flag) *Builder {
	b.app.Commands = append(b.app.Commands, &cli.Command{
		Name:  name,
		Usage: usage,
		Action: func(cc *cli.Context) error {
			b.runner.isMain = false
			parsedFlags := make(Flags)
			for _, flagName := range cc.LocalFlagNames() {
				parsedFlags[flagName] = cc.Value(flagName)
			}
			parsedArgs := Args(cc.Args().Slice())
			if err := callback(b.runner, parsedArgs, parsedFlags); err != nil {
				return err
			}
			return nil
		},
		Flags: flags,
	})
	return b
}

func (b *Builder) SubCommand(parent string, name string, usage string, callback Callback, flags ...cli.Flag) *Builder {
	var selectedCommand *cli.Command
	for _, command := range b.app.Commands {
		if command.Name == parent {
			selectedCommand = command
			break
		}
	}
	if selectedCommand == nil {
		selectedCommand = &cli.Command{
			Name: parent,
		}
		b.app.Commands = append(b.app.Commands, selectedCommand)
	}
	selectedCommand.Subcommands = append(selectedCommand.Subcommands, &cli.Command{
		Name:  name,
		Usage: usage,
		Action: func(cc *cli.Context) error {
			b.runner.isMain = false
			parsedFlags := make(Flags)
			for _, flagName := range cc.LocalFlagNames() {
				parsedFlags[flagName] = cc.Value(flagName)
			}
			parsedArgs := Args(cc.Args().Slice())
			if err := callback(b.runner, parsedArgs, parsedFlags); err != nil {
				return err
			}
			return nil
		},
		Flags: flags,
	})
	return b
}

func (b *Builder) Config(config interface{}) *Builder {
	b.config = config
	return b
}
