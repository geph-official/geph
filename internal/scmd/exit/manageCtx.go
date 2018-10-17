package exit

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/rensa-labs/geph/internal/niaucchi3"
	"golang.org/x/time/rate"
)

func (cmd *Command) getFuLimiter(uid userID, limit int) *rate.Limiter {
	cmd.fuTabLok.Lock()
	defer cmd.fuTabLok.Unlock()
	key := fmt.Sprintf("%v-%v", uid, limit)
	z := cmd.fuTab[key]
	if z == nil {
		z = rate.NewLimiter(rate.Limit(limit/20), 1500*1000*1000)
		cmd.fuTab[key] = z
	}
	return z
}

func (cmd *Command) manageCtx(uid userID, maxspeed int, ctx *niaucchi3.Context) {
	bal, err := cmd.decAccBalance(uid, 0)
	if err != nil {
		log.Println("error authenticating user", uid, ":", err)
	} else {
		log.Println(uid, "(NEW) connected with", bal, "MB; limit", maxspeed)
	}
	bal *= 1000000
	// little balance
	lbal := 0
	var lblk sync.Mutex
	// consume bytes, returns true if succeeds, otherwise returns false
	consume := func(dec int) bool {
		lblk.Lock()
		defer lblk.Unlock()
		lbal += dec
		bal -= dec
		if bal <= 0 {
			return false
		}
		return true
	}
	// periodically sync our balance with the global balance
	go func() {
		for {
			select {
			case <-ctx.Tomb().Dying():
				return
			case <-time.After(time.Second * 10):
				lblk.Lock()
				olbal := lbal
				lblk.Unlock()
				//if olbal > 1000000 {
				// olbal has what we need ATM
				// decrement by olbal
				nbal, err := cmd.decAccBalance(uid, (olbal / 1000000))
				if err != nil {
					log.Println("error", err.Error())
					nbal = 0
				}
				// then possibly decrement by another MiB, dithered
				if rand.Float64() < float64(olbal%1000000)/1000000.0 {
					nbal, err = cmd.decAccBalance(uid, 1)
					if err != nil {
						log.Println("error", err.Error())
						nbal = 0
					}
				}
				// update bal
				lblk.Lock()
				bal = nbal * 1000000
				lbal = 0
				lblk.Unlock()
				//}
			}
		}
	}()
	limit := rate.NewLimiter(rate.Limit(maxspeed*1000), 1000*1000)

	for {
		clnt, err := ctx.Accept()
		if err != nil {
			log.Println("exiting manageCtx:", err.Error())
			return
		}
		go cmd.proxyCommon(true, consume, limit, cmd.getFuLimiter(uid, maxspeed*1000), clnt)
	}
}
