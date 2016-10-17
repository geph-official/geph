package client

import "log"

// smFindEntry is the FindEntry state.
// => QueryBinder if cache is stale
// => ConnEntry if cache is fresh
func (cmd *Command) smFindEntry() {
	log.Println("** => FindEntry **")
	defer log.Println("** <= FindEntry **")

	if cmd.entryCache == nil {
		cmd.smState = cmd.smQueryBinder
	} else {
		cmd.smState = cmd.smConnEntry
	}
}
