package entry

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	tomb "gopkg.in/tomb.v2"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

var myHTTP = &http.Client{
	Transport: &http.Transport{
		TLSHandshakeTimeout: time.Second * 10,
		DisableKeepAlives:   true,
	},
	Timeout: time.Second * 10,
}

const cFRONT = "a0.awsstatic.com"
const cHOST = "dtnins2n354c4.cloudfront.net"

// Command is the entry subcommand.
type Command struct {
}

// Name returns the name "entry".
func (*Command) Name() string { return "entry" }

// Synopsis returns a description of the subcommand.
func (*Command) Synopsis() string { return "Run as the entry node" }

// Usage returns a string describing usage.
func (*Command) Usage() string { return "" }

// SetFlags sets the flag on the entry subcommand.
func (cmd *Command) SetFlags(f *flag.FlagSet) {
}

// Execute executes an entry subcommand.
func (cmd *Command) Execute(_ context.Context,
	f *flag.FlagSet,
	args ...interface{}) subcommands.ExitStatus {
	// our first step is to guess our own IP
lRETRY:
	resp, err := myHTTP.Get("http://icanhazip.com")
	if err != nil {
		log.Println("WARNING: stuck while getting our own IP:", err.Error(), "retrying in 30 secs")
		time.Sleep(time.Second * 30)
		goto lRETRY
	}
	buf := new(bytes.Buffer)
	io.Copy(buf, resp.Body)
	resp.Body.Close()
	myip := strings.Trim(string(buf.Bytes()), "\n ")
	log.Println("my own IP address guessed:", myip)
	// next, we enter the loop
	var choice string
	for {
		// we obtain the exit info first
		resp, err = myHTTP.Get("http://binder.geph.io/exit-info")
		if err != nil {
			log.Println("WARNING: stuck while getting exit info from binder, retrying")
			continue
		}
		io.Copy(buf, resp.Body)
		resp.Body.Close()
		var resp struct {
			Expires string
			Exits   map[string][]byte
		}
		err = json.Unmarshal(buf.Bytes(), &resp)
		if err != nil {
			log.Println("WARNING: bad json encountered in exit info, ignoring")
			continue
		}
		expTime, err := time.Parse(time.RFC3339, resp.Expires)
		if err != nil {
			log.Println("WARNING: bad time format in exit info, ignoring")
			continue
		}
		if expTime.Before(time.Now()) {
			log.Println("WARNING: expire time before now, ignoring")
			continue
		}
		log.Println("TODO: not checking sigs for exit info yet!")
		// we then see if our choice is in the given exits
		_, ok := resp.Exits[choice]
		if !ok {
			log.Println("beginning to race between the available exits...")
			tmb := new(tomb.Tomb)
			for dest, _ := range resp.Exits {

			}
		}
	}
}
