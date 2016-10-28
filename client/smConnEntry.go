package client

import (
	"io"
	"log"
	"time"

	"gopkg.in/bunsim/natrium.v1"

	"github.com/bunsim/geph/niaucchi"
)

// smConnEntry is the ConnEntry state, where a connection to some entry node is established.
// => VerifyAccount if successful
// => ClearCache if unsuccessful
func (cmd *Command) smConnEntry() {
	log.Println("** => ConnEntry **")
	defer log.Println("** <= ConnEntry **")

	FANOUT := 6
	if cmd.powersave {
		FANOUT = 1
	}

	retline := make(chan *niaucchi.Substrate)
	dedline := make(chan bool)
	for exit, entries := range cmd.entryCache {
		for _, xaxa := range entries {
			exit := exit
			xaxa := xaxa
			log.Println(xaxa.Addr, "from", exit)
			go func() {
				cand, merr := niaucchi.DialSubstrate(xaxa.Cookie,
					cmd.identity,
					xaxa.ExitKey.ToECDH(),
					xaxa.Addr, FANOUT)
				if merr != nil {
					log.Println(xaxa.Addr, "failed right away:", merr)
					return
				}
				select {
				case retline <- cand:
					cmd.stats.Lock()
					cmd.stats.netinfo.entry = natrium.HexEncode(
						natrium.SecureHash(xaxa.Cookie, nil)[:8])
					cmd.stats.netinfo.exit = exit
					cmd.stats.netinfo.prot = "cl-ni-1"
					cmd.stats.Unlock()
					log.Println(xaxa.Addr, "WINNER")
				case <-dedline:
					log.Println(xaxa.Addr, "failed race")
					cand.Tomb().Kill(io.ErrClosedPipe)
				}
			}()
		}
	}

	select {
	case <-time.After(time.Second * 10):
		log.Println("ConnEntry: failed to connect to anything within 10 seconds")
		cmd.smState = cmd.smClearCache
		return
	case ss := <-retline:
		close(dedline)
		cmd.currTunn = ss
		cmd.smState = cmd.smVerifyAccount
		return
	}
}

// smClearCache clears the cache and goes back to the entry point.
// => FindEntry always
func (cmd *Command) smClearCache() {
	log.Println("** => ClearCache **")
	defer log.Println("** <= ClearCache **")
	cmd.entryCache = nil
	cmd.exitCache = nil
	cmd.smState = cmd.smFindEntry
	if cmd.cdb != nil {
		cmd.cdb.Exec("DELETE FROM main WHERE k='bst.entries'")
	}
}
