package niaucchi2

const (
	flOpen = 0x01
	flClos = 0x02

	flData = 0x10
	flIcwd = 0x11

	flAliv = 0xff
)

type segment struct {
	Flag   uint8
	Sokid  uint16
	BodLen uint16 `struc:"uint16,sizeof=Body"`
	Body   []byte
}
