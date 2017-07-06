package main

import (
	"flag"
	"math/rand"
	"os"

	"golang.org/x/net/context"

	"github.com/google/subcommands"
	"github.com/rensa-labs/geph/binder"
	"github.com/rensa-labs/geph/client"
	"github.com/rensa-labs/geph/entry"
	"github.com/rensa-labs/geph/exit"
	"github.com/rensa-labs/geph/proxbinder"
	"gopkg.in/bunsim/natrium.v1"
)

func main() {
	// seed the insecure RNG from the secure one to prevent repeats
	rand.Seed(int64(natrium.RandUint32()))
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&binder.Command{}, "")
	subcommands.Register(&exit.Command{}, "")
	subcommands.Register(&entry.Command{}, "")
	subcommands.Register(&client.Command{}, "")
	subcommands.Register(&proxbinder.Command{}, "")
	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
