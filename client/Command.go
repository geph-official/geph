package client

import (
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"gopkg.in/bunsim/natrium.v1"

	"github.com/bunsim/geph/niaucchi"
	"github.com/bunsim/goproxy"
	"github.com/google/subcommands"
	"golang.org/x/net/context"

	// SQLite3
	_ "github.com/mattn/go-sqlite3"

	// pprof
	_ "net/http/pprof"
)

const cFRONT = "a0.awsstatic.com"
const cHOST = "dtnins2n354c4.cloudfront.net"

var binderPub natrium.EdDSAPublic

func init() {
	binderPub, _ = natrium.HexDecode("d25bcdc91961a6e9e6c74fbcd5eb977c18e7b1fe63a78ec62378b55aa5172654")
}

var cleanHTTP = &http.Client{
	Transport: &http.Transport{
		TLSHandshakeTimeout: time.Second * 10,
		IdleConnTimeout:     time.Second * 10,
	},
	Timeout: time.Second * 10,
}

type entryInfo struct {
	Addr    string
	Cookie  []byte
	ExitKey natrium.EdDSAPublic
}

// Command is the client subcommand.
type Command struct {
	uname     string
	pwd       string
	identity  natrium.ECDHPrivate
	cachedir  string
	powersave bool

	cdb        *sql.DB
	exitCache  map[string][]byte
	entryCache map[string][]entryInfo
	currTunn   *niaucchi.Substrate

	proxtrans  *http.Transport
	proxclient *http.Client

	stats struct {
		status  string
		rxBytes uint64
		txBytes uint64
		stTime  time.Time

		netinfo struct {
			exit  string
			entry string
			prot  string
			tuns  map[string]string
		}

		sync.RWMutex
	}

	smState func()
}

func touid(b []byte) string {
	uid := strings.ToLower(
		base32.StdEncoding.EncodeToString(
			natrium.SecureHash(b, nil)[:10]))
	return uid
}

// Name returns the name "client".
func (*Command) Name() string { return "client" }

// Synopsis returns a description of the subcommand.
func (*Command) Synopsis() string { return "Run as the client" }

// Usage returns a string describing usage.
func (*Command) Usage() string { return "" }

// SetFlags sets the flag on the binder subcommand.
func (cmd *Command) SetFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.uname, "uname", "test", "username")
	f.StringVar(&cmd.pwd, "pwd", "removekebab", "password")
	f.StringVar(&cmd.cachedir, "cachedir", "", "cache directory; if empty then no cache is used")
	f.BoolVar(&cmd.powersave, "powersave", false, "optimize for saving power on mobile devices, at the cost of some performance")
}

// Execute executes a client subcommand.
func (cmd *Command) Execute(_ context.Context,
	f *flag.FlagSet,
	args ...interface{}) subcommands.ExitStatus {
	// Initialize stats
	cmd.stats.status = "connecting"
	cmd.stats.stTime = time.Now()
	cmd.stats.netinfo.tuns = make(map[string]string)
	// spawn the RPC servers
	go func() {
		http.HandleFunc("/proxy.pac", cmd.servPac)
		http.HandleFunc("/summary", cmd.servSummary)
		http.HandleFunc("/accinfo", cmd.servAccInfo)
		http.HandleFunc("/netinfo", cmd.servNetinfo)
		err := http.ListenAndServe("127.0.0.1:8790", nil)
		if err != nil {
			panic(err.Error())
		}
	}()
	// set up proxtrans
	cmd.proxtrans = &http.Transport{
		Dial: func(n, d string) (net.Conn, error) {
			return cmd.dialTun(d)
		},
		IdleConnTimeout: time.Second * 10,
	}
	cmd.proxclient = &http.Client{
		Transport: cmd.proxtrans,
		Timeout:   time.Second * 10,
	}
	// spawn the SOCKS5 server
	socksListener, err := net.Listen("tcp", "127.0.0.1:8781")
	if err != nil {
		panic(err.Error())
	}
	go cmd.doSocks(socksListener)
	// try to connect to the cache first
	if cmd.cachedir != "" {
		var err error
		cmd.cdb, err = sql.Open("sqlite3", fmt.Sprintf("%v/%x.db", cmd.cachedir,
			natrium.SecureHash([]byte(cmd.uname), []byte(cmd.pwd))[:8]))
		if err != nil {
			panic(err.Error())
		}
		// just a simple key-value pair lol
		cmd.cdb.Exec("CREATE TABLE IF NOT EXISTS main (k UNIQUE NOT NULL, v)")
	}
	// Try to read the identity from the cache first
	if cmd.cdb != nil {
		row := cmd.cdb.QueryRow("SELECT v FROM main WHERE k = 'sec.identity'")
		var lol []byte
		err := row.Scan(&lol)
		if err != nil {
			log.Println("cache: cannot read sec.identity:", err.Error())
		} else {
			cmd.identity = lol
			log.Println("identity (cache):", touid(cmd.identity.PublicKey()))
		}
	}
	// Derive the identity
	if cmd.identity == nil {
		prek := natrium.SecureHash([]byte(cmd.pwd), []byte(cmd.uname))
		cmd.identity = natrium.EdDSADeriveKey(
			natrium.StretchKey(prek, make([]byte, natrium.PasswordSaltLen), 8, 64*1024*1024)).ToECDH()
		// Place identity in cache if available
		if cmd.cdb != nil {
			_, err := cmd.cdb.Exec("INSERT INTO main VALUES ('sec.identity', $1)", []byte(cmd.identity))
			if err != nil {
				log.Println("cache: cannot store sec.identity:", err.Error())
			}
		}
		log.Println("identity (deriv):", touid(cmd.identity.PublicKey()))
		j, _ := json.Marshal(cmd.identity)
		log.Println("identity (cache):", string(j))
	}
	// Start the DNS daemon which should never stop
	go cmd.doDNS()
	// Start the HTTP which should never stop
	// spawn the HTTP proxy server
	srv := goproxy.NewProxyHttpServer()
	srv.Tr = cmd.proxtrans
	srv.Logger = log.New(ioutil.Discard, "", 0)
	go func() {
		err := http.ListenAndServe("127.0.0.1:8780", srv)
		if err != nil {
			panic(err.Error())
		}
	}()
	// Start the state machine in smFindEntry
	cmd.smState = cmd.smFindEntry
	for {
		cmd.smState()
	}
}
