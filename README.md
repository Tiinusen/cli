# CLI (wrapper)

Wraps urfave/cli and provides some ease of use functions allowing method chaining pattern

## Example

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/tiinusen/cli"
)

func main() {
	app, err := cli.New("name-of-binary", "usage description", "v1.0.0").
		Daemonize("name-of-binary", "service description").
		Config(config{}.Defaults()).
		Command("command", "command description", func(c *cli.Runner, args cli.Args, flags cli.Flags) error {
			log.Println("Hello A: ", flags.Integer("my-int-flag"))
			fmt.Println(c.Config().(config))
			return nil
		}, cli.IntegerFlag("my-int-flag", "flag description")).
		SubCommand("command", "subcommand", "sub command description", func(c *cli.Runner, args cli.Args, flags cli.Flags) error {
			log.Println("Hello B")
			return nil
		}).
		Run(os.Args...)
	if err != nil {
		app.Exit(err)
	}
}

type config struct {
	MyIntVar               int `help:"my int help" flag:"my-int-var,miv"`
	MyStringVar            string
	MyStringVarWithDefault string
	MyBooleanVar           bool
	MyInnerStruct          struct {
		MyInnerInt    int
		MyInnerString string
	}
}

func (c config) Defaults() (b config) {
	return config{
		MyStringVar: "test",
	}
}
```

## Request
If package is missing some vital feature, one can always request it, but better to do it and submit a pull request