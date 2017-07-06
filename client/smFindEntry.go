package client

import (
	"log"
	"time"
)

// smFindEntry is the FindEntry state.
// => QueryBinder if cache is stale
// => ConnEntry if cache is fresh
func (cmd *Command) smFindEntry() {
	log.Println("** => FindEntry **")
	defer log.Println("** <= FindEntry **")
	// is the cache empty
	if cmd.ecache.GetEntries() == nil {
		exits, err := cmd.getExitNodes()
		if err != nil {
			time.Sleep(time.Second)
			cmd.smState = cmd.smFindEntry
			return
		}
		log.Println(exits)
		entries := cmd.getEntryNodes(exits)
		if len(entries) == 0 {
			log.Println("no entries found!")
			time.Sleep(time.Second)
			cmd.smState = cmd.smFindEntry
			return
		}
		cmd.ecache.SetEntries(entries)
		cmd.smState = cmd.smConnEntry
		return
	}
	// asynchronously update the cache anyway
	go func() {
		exits, err := cmd.getExitNodes()
		if err != nil {
			return
		}
		entries := cmd.getEntryNodes(exits)
		if len(entries) == 0 {
			return
		}
		cmd.ecache.SetEntries(entries)
	}()
	cmd.smState = cmd.smConnEntry
	return
}
