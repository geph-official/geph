package proxbinder

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"context"

	"github.com/google/subcommands"
)

// Command is the proxbinder subcommand.
type Command struct {
}

// Name returns the name "proxbinder".
func (*Command) Name() string { return "proxbinder" }

// Synopsis returns a description of the subcommand.
func (*Command) Synopsis() string {
	return "Utility that proxies the binder (and does some other things)"
}

// Usage returns a string describing usage.
func (*Command) Usage() string {
	return "proxbinder mirrors the binder on localhost. This might be useful for UIs etc who do not want to reimplement domain fronting. In addition, it offers a key-deriving service at /derive-keys?uname=..&pwd=.. to help account registration. It prints out to localhost the address it's listening on."
}

// SetFlags sets the flag on the binder subcommand.
func (cmd *Command) SetFlags(f *flag.FlagSet) {
}

const cFRONT = "a0.awsstatic.com"
const cHOST = "dtnins2n354c4.cloudfront.net"

// Execute executes a binder subcommand.
func (cmd *Command) Execute(_ context.Context,
	f *flag.FlagSet,
	args ...interface{}) subcommands.ExitStatus {
	rp := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "http"
			r.URL.Host = cFRONT
			r.Host = cHOST
			log.Println("reverse proxying", r.URL)
		},
	}
	mux := &http.ServeMux{}
	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second * 5,
	}
	mux.Handle("/", rp)
	mux.HandleFunc("/derive-keys", cmd.handDeriveKeys)
	lsnr, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(lsnr.Addr())
	srv.Serve(lsnr)
	return 0
}
