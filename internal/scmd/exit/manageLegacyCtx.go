package exit

import (
	"io"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/rensa-labs/geph/internal/legacy/niaucchi2"
	"golang.org/x/time/rate"
)

func (cmd *Command) manageLegacyCtx(uid string, nct *niaucchi2.Context) {
	defer nct.Tomb().Kill(io.ErrClosedPipe)
	// check balance first
	bal, err := cmd.decLegacyAccBalance(uid, 0)
	if err != nil {
		log.Println("error authenticating user", uid, ":", err)
	} else {
		log.Println(uid, "(LEGACY) connected with", bal, "MiB left")
	}
	bal *= 1000000
	limit := rate.NewLimiter(rate.Limit(cmd.bwLimit*1024), 20*1024*1024)
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
			case <-nct.Tomb().Dying():
				return
			case <-time.After(time.Second * 10):
				lblk.Lock()
				olbal := lbal
				lblk.Unlock()
				//if olbal > 1000000 {
				// olbal has what we need ATM
				// decrement by olbal
				nbal, err := cmd.decLegacyAccBalance(uid, (olbal / 1000000))
				if err != nil {
					log.Println("error", err.Error())
					nbal = 0
				}
				// then possibly decrement by another MiB, dithered
				if rand.Float64() < float64(olbal%1000000)/1000000.0 {
					nbal, err = cmd.decLegacyAccBalance(uid, 1)
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
	// Accept loop
	for {
		clnt, err := nct.Accept()
		if err != nil {
			log.Println("exiting manageLegacyCtx:", err.Error())
			return
		}
		go cmd.proxyCommon(true, consume, limit, clnt)
	}
}
