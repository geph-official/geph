package client

import (
	"encoding/binary"
	"encoding/csv"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
)

type geoDBEntry struct {
	from uint
	to   uint
	ctry string
}

type geoDB struct {
	slice []geoDBEntry
}

func (gdb *geoDB) LoadCSV(fname string) (err error) {
	fd, err := os.Open(fname)
	if err != nil {
		return
	}
	defer fd.Close()
	rdr := csv.NewReader(fd)
	for {
		var rcd []string
		rcd, err = rdr.Read()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		if len(rcd) != 6 {
			return errors.New("malformed csv")
		}
		s, _ := strconv.Atoi(rcd[2])
		e, _ := strconv.Atoi(rcd[3])
		c := rcd[4]
		gdb.slice = append(gdb.slice, geoDBEntry{uint(s), uint(e), c})
	}
}

func (gdb *geoDB) GetCountry(ip net.IP) string {
	// convert IP to integer
	ipint := uint(binary.BigEndian.Uint32(ip[12:]))
	// binary search
	sl := gdb.slice
	lo := 0
	hi := len(gdb.slice)
	for {
		mid := (lo + hi) / 2
		elem := sl[mid]
		if elem.from <= ipint && ipint <= elem.to {
			return elem.ctry
		} else if lo == mid || mid == hi {
			return "UN"
		} else if ipint < elem.from {
			hi = mid
		} else if ipint > elem.to {
			lo = mid
		} else {
			panic("this should never happen")
		}
	}
}
