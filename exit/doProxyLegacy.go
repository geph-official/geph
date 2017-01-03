package exit

import (
	"encoding/base32"
	"io"
	"log"
	"math/rand"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"github.com/bunsim/geph/niaucchi"
	"gopkg.in/bunsim/natrium.v1"
)

// TODO refactor this function, it's getting too messy
func (cmd *Command) doProxyLegacy() {
	lsnr, err := niaucchi.Listen(nil, cmd.identity.ToECDH(), ":2378")
	if err != nil {
		panic(err.Error())
	}
	log.Println("niaucchi listening on port 2378")
	for {
		ss, err := lsnr.AcceptSubstrate()
		if err != nil {
			panic(err.Error())
		}
		go func() {
			defer ss.Tomb().Kill(io.ErrClosedPipe)
			// get their pk first
			pub := ss.RemotePK()
			uid := strings.ToLower(
				base32.StdEncoding.EncodeToString(
					natrium.SecureHash(pub, nil)[:10]))
			// check balance first
			bal, err := cmd.decAccBalance(uid, 0)
			if err != nil {
				log.Println("error authenticating user", uid, ":", err)
			} else {
				log.Println(uid, "connected with", bal, "MiB left")
			}
			limit := rate.NewLimiter(rate.Limit(cmd.bwLimit*1024), 512*1024)
			harshlimit := rate.NewLimiter(rate.Limit(32*1024), 128*1024)
			// little balance
			lbal := 0
			var lblk sync.Mutex
			// consume bytes, returns true if succeeds, otherwise returns false and kills everything
			consume := func(dec int) bool {
				lblk.Lock()
				defer lblk.Unlock()
				lbal -= dec
				if lbal <= 0 {
					num := rand.Int()%10 + 5
					bal, err := cmd.decAccBalance(uid, num)
					if err != nil || bal == 0 {
						return false
					}
					lbal += 1000 * 1000 * num
				}
				return true
			}
			// at the very end, return the small balance
			defer func() {
				cmd.decAccBalance(uid, -lbal/1000000)
			}()
			for {
				clnt, _, err := ss.AcceptConn()
				if err != nil {
					return
				}
				go cmd.proxyCommon(false, consume, limit, harshlimit, uid, clnt)
			}
		}()
	}
}
