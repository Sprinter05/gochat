package gcspec

type packet struct {
	hdr     Header
	len     Length
	payload []byte
}
