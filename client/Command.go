package client

import (
	"crypto/tls"
	"flag"
	"net"
	"net/http"
	"sync"
	"time"

	"gopkg.in/bunsim/natrium.v1"

	"github.com/bunsim/geph/niaucchi"
	"github.com/bunsim/goproxy"
	"github.com/google/subcommands"
	"golang.org/x/net/context"
	"golang.org/x/net/proxy"
)

const cFRONT = "a0.awsstatic.com"
const cHOST = "dtnins2n354c4.cloudfront.net"

var binderPub natrium.EdDSAPublic

func init() {
	binderPub, _ = natrium.HexDecode("d25bcdc91961a6e9e6c74fbcd5eb977c18e7b1fe63a78ec62378b55aa5172654")
}

var myHTTP = &http.Client{
	Transport: &http.Transport{
		TLSHandshakeTimeout: time.Second * 10,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		//DisableKeepAlives:   true,
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
	uname    string
	pwd      string
	identity natrium.ECDHPrivate

	exitCache  map[string][]byte
	entryCache map[string][]entryInfo
	currTunn   *niaucchi.Substrate

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
}

// Execute executes a client subcommand.
func (cmd *Command) Execute(_ context.Context,
	f *flag.FlagSet,
	args ...interface{}) subcommands.ExitStatus {
	// Derive the identity first
	prek := natrium.SecureHash([]byte(cmd.pwd), []byte(cmd.uname))
	cmd.identity = natrium.EdDSADeriveKey(
		natrium.StretchKey(prek, make([]byte, natrium.PasswordSaltLen), 8, 64*1024*1024)).ToECDH()
	// Initialize stats
	cmd.stats.status = "connecting"
	cmd.stats.stTime = time.Now()
	cmd.stats.netinfo.tuns = make(map[string]string)
	// Start the DNS daemon which should never stop
	go cmd.doDNS()
	// Start the HTTP which should never stop
	// spawn the HTTP proxy server
	srv := goproxy.NewProxyHttpServer()
	srv.Tr = &http.Transport{
		Dial: func(n, d string) (net.Conn, error) {
			dler, err := proxy.SOCKS5("tcp", "localhost:8781", nil, proxy.Direct)
			if err != nil {
				panic(err.Error())
			}
			return dler.Dial(n, d)
		},
		MaxIdleConns: 0,
	}
	// spawn the RPC servers
	go func() {
		http.HandleFunc("/summary", cmd.servSummary)
		http.HandleFunc("/netinfo", cmd.servNetinfo)
		err := http.ListenAndServe("127.0.0.1:8790", nil)
		if err != nil {
			panic(err.Error)
		}
	}()
	go func() {
		err := http.ListenAndServe("127.0.0.1:8780", srv)
		if err != nil {
			panic(err.Error)
		}
	}()
	// Start the state machine in smFindEntry
	cmd.smState = cmd.smFindEntry
	for {
		cmd.smState()
	}
}
