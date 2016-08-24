package binder

import (
	"flag"
	"log"
	"net/http"

	"github.com/ProjectNiwl/natrium"
	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

// Command is the binder subcommand.
type Command struct {
	idSeed   string
	exitConf string

	identity natrium.EdDSAPrivate
}

// Name returns the name "binder".
func (*Command) Name() string { return "binder" }

// Synopsis returns a description of the subcommand.
func (*Command) Synopsis() string { return "Run as the binder" }

// Usage returns a string describing usage.
func (*Command) Usage() string { return "" }

// SetFlags sets the flag on the binder subcommand.
func (cmd *Command) SetFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.idSeed, "idSeed", "", "seed to use to generate private key")
	f.StringVar(&cmd.exitConf, "exitConf", "exitconf.json",
		"JSON config file containing the exit servers")
}

// Execute executes a binder subcommand.
func (cmd *Command) Execute(_ context.Context,
	f *flag.FlagSet,
	args ...interface{}) subcommands.ExitStatus {
	// validate
	if cmd.idSeed == "" {
		panic("idSeed must be given")
	}
	// generate the real stuff from the flags
	cmd.identity = natrium.EdDSADeriveKey([]byte(cmd.idSeed))
	log.Println("Binder started; public key is", natrium.HexEncode(cmd.identity.PublicKey()))
	// run the stuff
	http.HandleFunc("/exit-info", cmd.handExitInfo)
	http.ListenAndServe(":8080", nil)
	return 0
}
