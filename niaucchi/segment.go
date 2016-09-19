package niaucchi

const (
	flData = 0x00
	flOpen = 0x01
	flClos = 0x02

	flAliv = 0x10

	flAck = 0xff
)

type segment struct {
	Flag   uint8
	ConnID uint16
	Serial uint64
	BodLen int `struc:"uint32,sizeof=Body"`
	Body   []byte
}
