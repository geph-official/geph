package binder

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
	"gopkg.in/bunsim/natrium.v1"

	// postgres
	_ "github.com/lib/pq"
)

// Command is the binder subcommand.
type Command struct {
	idSeed   string
	exitConf string

	identity natrium.EdDSAPrivate

	pgdb *sql.DB
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
	// connect to the postgres
	var err error
	cmd.pgdb, err = sql.Open("postgres",
		"postgres://postgres:postgres@localhost/postgres?sslmode=disable")
	if err != nil {
		panic(err.Error())
	}
	// generate the real stuff from the flags
	cmd.identity = natrium.EdDSADeriveKey([]byte(cmd.idSeed))
	log.Println("Binder started; public key is", natrium.HexEncode(cmd.identity.PublicKey()))
	log.Println("Listening on 127.0.0.1:8080. Please set up nginx or a similar reverse proxy to provide service on ports 80 and 443.")
	// run the stuff
	http.HandleFunc("/exit-info", cmd.handExitInfo)
	http.HandleFunc("/account-summary", cmd.handAccountSummary)
	rp := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			frags := strings.Split(r.URL.Path, "/")
			r.URL.Scheme = "http"
			r.URL.Host = fmt.Sprintf("%v", frags[2])
			r.URL.Path = "/" + strings.Join(frags[3:], "/")
			r.Host = r.URL.Host
			log.Println("reverse proxying", r.URL)
		},
	}
	http.Handle("/exits/", rp)
	if http.ListenAndServe("127.0.0.1:8080", nil) != nil {
		return -1
	}
	return 0
}
