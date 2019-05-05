package entry

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"context"

	"github.com/google/subcommands"
	"gopkg.in/bunsim/natrium.v1"
)

var myHTTP = &http.Client{
	Transport: &http.Transport{
		TLSHandshakeTimeout: time.Second * 10,
		DisableKeepAlives:   true,
	},
	Timeout: time.Second * 10,
}

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
	rand.Seed(time.Now().UnixNano())
	// we enter the loop
	resp, err := myHTTP.Get("https://ipv4.icanhazip.com")
	if err != nil {
		panic("stuck while getting our own IP: " + err.Error())
	}
	buf := new(bytes.Buffer)
	io.Copy(buf, resp.Body)
	resp.Body.Close()
	myip := strings.Trim(string(buf.Bytes()), "\n ")
	log.Println("my own IP address guessed:", myip)
	for {
		// we obtain the exit info first
		resp, err = myHTTP.Get("https://binder.geph.io/exit-info")
		if err != nil {
			log.Println("WARNING: stuck while getting exit info from binder:", err.Error())
			continue
		}
		buf.Reset()
		io.Copy(buf, resp.Body)
		resp.Body.Close()
		var exinf struct {
			Expires string
			Exits   map[string][]byte
		}
		err = json.Unmarshal(buf.Bytes(), &exinf)
		if err != nil {
			log.Println("WARNING: bad json encountered in exit info:", err.Error())
			continue
		}
		expTime, err := time.Parse(time.RFC3339, exinf.Expires)
		if err != nil {
			log.Println("WARNING: bad time format in exit info, ignoring")
			continue
		}
		if expTime.Before(time.Now()) {
			log.Println("WARNING: expire time before now, ignoring")
			continue
		}
		for choice := range exinf.Exits {
			cookie := make([]byte, 12)
			natrium.RandBytes(cookie)
			lsnr, err := net.Listen("tcp4", ":0")
			if err != nil {
				panic(err)
			}
			go cmd.doForward(lsnr, cookie, choice)
			// we then do our upload
			var tosend struct {
				Addr   string
				Cookie []byte
			}
			tosend.Addr = fmt.Sprintf("%v:%v", myip, lsnr.Addr().(*net.TCPAddr).Port)
			log.Printf("reverse-proxy %v => %v", tosend.Addr, choice)
			tosend.Cookie = cookie
			bts, _ := json.Marshal(tosend)
			for {
				resp, err = myHTTP.Post(fmt.Sprintf("http://%v:8081/update-node", choice),
					"application/json",
					bytes.NewReader(bts))
				if err != nil {
					log.Println("WARNING: failed uploading entry info to", choice)
				} else {
					resp.Body.Close()
				}
				time.Sleep(time.Second * 30)
			}
		}
		time.Sleep(time.Hour)
	}
}
