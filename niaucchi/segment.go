package niaucchi

const (
	flData = 0x00
	flOpen = 0x01
	flClos = 0x02
	// FastOpen is not acknowledged, and can contain additional data in the body.
	flFastOpen = 0x03

	flAliv = 0x10

	flAck = 0xff
)

type segment struct {
	Flag   uint8
	ConnID uint16
	Serial uint64
	BodLen int `struc:"uint16,sizeof=Body"`
	Body   []byte
}
