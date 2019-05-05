package exit

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"context"

	"github.com/google/subcommands"
	"golang.org/x/time/rate"
	"gopkg.in/bunsim/natrium.v1"

	// postgres
	_ "github.com/lib/pq"

	// pprof
	_ "net/http/pprof"
)

// Command is the exit subcommand.
type Command struct {
	idSeed  string
	bwLimit int
	pgURL   string

	wfFront string
	wfHost  string

	identity natrium.EdDSAPrivate
	edb      *entryDB

	fuTab    map[string]*rate.Limiter
	fuTabLok sync.Mutex

	pgdb *sql.DB
}

// Name returns the name "exit".
func (*Command) Name() string { return "exit" }

// Synopsis returns a description of the subcommand.
func (*Command) Synopsis() string { return "Run as an exit" }

// Usage returns a string describing usage.
func (*Command) Usage() string { return "" }

// SetFlags sets the flag on the binder subcommand.
func (cmd *Command) SetFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.idSeed, "idSeed", "", "seed to use to generate private key")
	f.StringVar(&cmd.pgURL, "pgURL", "127.0.0.1:15432",
		"location of the PostgreSQL account database")
	f.IntVar(&cmd.bwLimit, "bwLimit", 2000, "bandwidth limit for every session (KiB/s)")

	f.StringVar(&cmd.wfFront, "wfFront", "", "front for warpfront")
	f.StringVar(&cmd.wfHost, "wfHost", "", "host for warpfront")
}

// Execute executes the exit subcommand.
func (cmd *Command) Execute(_ context.Context,
	f *flag.FlagSet,
	args ...interface{}) subcommands.ExitStatus {
	cmd.fuTab = make(map[string]*rate.Limiter)
	// validate
	if cmd.idSeed == "" {
		panic("idSeed must be given")
	}

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// generate the real stuff from the flags
	cmd.identity = natrium.EdDSADeriveKey([]byte(cmd.idSeed))
	log.Println("idSeed is", cmd.idSeed)
	b64, _ := json.Marshal(cmd.identity.PublicKey())
	log.Println("** Public key is", string(b64), "**")
	cmd.edb = newEntryDB("/tmp/geph-exit-cache.db")

	// connect to the PostgreSQL database
	db, err := sql.Open("postgres",
		fmt.Sprintf("postgres://postgres:postgres@%v/postgres?sslmode=disable", cmd.pgURL))
	if err != nil {
		panic(err.Error())
	}
	cmd.pgdb = db

	// run the proxy
	go cmd.doProxy()
	mux := http.NewServeMux()
	hserv := &http.Server{
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        mux,
		Addr:           ":8081",
	}

	// run the exit API
	mux.HandleFunc("/update-node", cmd.handUpdateNode)
	mux.HandleFunc("/get-nodes", cmd.handGetNodes)
	mux.HandleFunc("/test-speed", cmd.handTestSpeed)
	hserv.ListenAndServe()
	return 0
}
