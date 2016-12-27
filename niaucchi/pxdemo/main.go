package main

import (
	"flag"
	"os"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&serverCmd{}, "")
	subcommands.Register(&clientCmd{}, "")
	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
