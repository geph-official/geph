package niaucchi

import (
	"errors"
	"sync"
)

// sorter sorts things in real time
type sorter struct {
	cue map[uint64]interface{}
	ptr uint64
	lok sync.Mutex
}

func newSorter() *sorter {
	return &sorter{
		cue: make(map[uint64]interface{}),
	}
}

// Push pushes an object with a particular position
func (srt *sorter) Push(pos uint64, obj interface{}) error {
	srt.lok.Lock()
	defer srt.lok.Unlock()
	if pos-srt.ptr > 3000 || pos < srt.ptr {
		return errors.New("desync")
	}
	//log.Println("niaucchi: sorter at", srt.ptr, "with", len(srt.cue), "outstanding")
	srt.cue[pos] = obj
	return nil
}

// Pop tries to advance the position as far as possible, popping the things
func (srt *sorter) Pop() []interface{} {
	srt.lok.Lock()
	defer srt.lok.Unlock()
	var toret []interface{}
	for {
		obj, ok := srt.cue[srt.ptr]
		if !ok {
			return toret
		}
		toret = append(toret, obj)
		delete(srt.cue, srt.ptr)
		srt.ptr++
	}
}
