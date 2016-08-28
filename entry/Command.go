package entry

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ProjectNiwl/natrium"
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
	// we enter the loop
	var choice string
	cookie := make([]byte, 12)
	natrium.RandBytes(cookie)
	lsnr, _ := net.ListenTCP("tcp", nil)
	go cmd.doForward(lsnr, cookie, &choice)
	for {
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
		// we obtain the exit info first
		resp, err = myHTTP.Get("http://binder.geph.io:8080/exit-info")
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
		log.Println("TODO: not checking sigs for exit info yet!")
		// we then see if our choice is in the given exits
		_, ok := exinf.Exits[choice]
		if !ok {
			log.Println("beginning to race between the available exits...")
			speeds := make(map[string]float64)
			lats := make(map[string]float64)
			for dest := range exinf.Exits {
				t1 := time.Now()
				resp, err = myHTTP.Get(fmt.Sprintf("http://%v:8081/test-speed", dest))
				if err != nil {
					log.Println("speed test TOTALLY FAILED for", dest)
					continue
				}
				t2 := time.Now()
				buf.Reset()
				io.Copy(buf, resp.Body)
				resp.Body.Close()
				t3 := time.Now()
				lats[dest] = t2.Sub(t1).Seconds()
				speeds[dest] = 8 / t3.Sub(t2).Seconds()
				log.Println(dest, "has latency", t2.Sub(t1),
					"and throughput", 8/t3.Sub(t2).Seconds(), "Mbps")
			}
			for k, v := range speeds {
				if v > speeds[choice] {
					choice = k
				}
			}
			log.Println("our choice of exit is", choice, "based purely on throughput")
		}
		// we then do our upload
		var tosend struct {
			Addr   string
			Cookie []byte
		}
		tosend.Addr = fmt.Sprintf("%v:%v", myip, lsnr.Addr().(*net.TCPAddr).Port)
		tosend.Cookie = cookie
		bts, _ := json.Marshal(tosend)
		fmt.Println(string(bts))
		resp, err = myHTTP.Post(fmt.Sprintf("http://%v:8081/update-node", choice),
			"application/json",
			bytes.NewReader(bts))
		if err != nil {
			log.Println("WARNING: failed uploading entry info to", choice)
		} else {
			log.Println("uploaded entry info to", choice)
			resp.Body.Close()
		}
		time.Sleep(time.Minute * 2)
	}
}
