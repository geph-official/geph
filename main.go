package main

import (
	"flag"
	"os"

	"golang.org/x/net/context"

	"github.com/bunsim/geph/binder"
	"github.com/bunsim/geph/exit"
	"github.com/google/subcommands"
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&binder.Command{}, "")
	subcommands.Register(&exit.Command{}, "")
	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
