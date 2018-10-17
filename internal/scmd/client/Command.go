package client

import (
	"database/sql"
	"encoding/base32"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/bunsim/natrium.v1"

	"context"

	"github.com/bunsim/goproxy"
	"github.com/google/subcommands"
	"github.com/rensa-labs/geph/internal/common"
	"github.com/rensa-labs/geph/internal/niaucchi3"

	// SQLite3
	_ "gopkg.in/mattn/go-sqlite3.v1"
)

var cFRONTs = []string{
	"d1u5p3l4wpay3k.cloudfront.net",
	"d1u5p3l4wpay3k.cloudfront.net",
	"d1u5p3l4wpay3k.cloudfront.net",
	"d1u5p3l4wpay3k.cloudfront.net",
	"cloudfront.debian.net",
	"cloudfront.debian.net",
	"cloudfront.debian.net",
	"cloudfront.debian.net",
	"cloudfront.debian.net",
	"cloudfront.debian.net",
	"dtnins2n354c4.cloudfront.net"}

const cHOST = "dtnins2n354c4.cloudfront.net"

var binderPub natrium.EdDSAPublic

func init() {
	binderPub, _ = natrium.HexDecode("d25bcdc91961a6e9e6c74fbcd5eb977c18e7b1fe63a78ec62378b55aa5172654")
}

var cleanHTTP = &http.Client{
	Transport: &http.Transport{
		Proxy:               nil,
		TLSHandshakeTimeout: time.Second * 30,
		IdleConnTimeout:     time.Second * 10,
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 16,
	},
	Timeout: time.Second * 200,
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

	cdb      *sql.DB
	ecache   entryCache
	currTunn *niaucchi3.Context

	geodb    geoDB
	whitegeo []string
	wliststr string
	geodbloc string

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
	f.BoolVar(&cmd.powersave, "powersave", true,
		"optimize for saving power on mobile devices, at the cost of some performance")

	f.StringVar(&cmd.wliststr, "whitelist", "", "comma-separated countries to not proxy (example: \"CN,US\")")
	f.StringVar(&cmd.geodbloc, "geodb", "",
		"location of GeoIP database; must be given if countries are to be whitelisted")
}

// Execute executes a client subcommand.
func (cmd *Command) Execute(_ context.Context,
	f *flag.FlagSet,
	args ...interface{}) subcommands.ExitStatus {
	os.Setenv("HTTP_PROXY", "")
	os.Setenv("HTTPS_PROXY", "")
	// Initialize stats
	cmd.stats.status = "connecting"
	cmd.stats.stTime = time.Now()
	cmd.stats.netinfo.tuns = make(map[string]string)
	// Initialize GeoIP
	if cmd.geodbloc != "" {
		var err error
		cmd.whitegeo = strings.Split(cmd.wliststr, ",")
		err = cmd.geodb.LoadCSV(cmd.geodbloc)
		if err != nil {
			panic(err.Error())
		}
	}
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
			log.Println("GONNA DIAL", n, d)
			return cmd.dialTun(d)
		},
		IdleConnTimeout: time.Second * 10,
		Proxy:           nil,
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
		cmd.ecache = &sqliteEntryCache{cmd.cdb}
	} else {
		cmd.ecache = &memEntryCache{}
	}
	// Derive the identity
	if cmd.identity == nil {
		cmd.identity = common.DeriveKey(cmd.uname, cmd.pwd).ToECDH()
		log.Println("identity (deriv):", touid(cmd.identity.PublicKey()))
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
