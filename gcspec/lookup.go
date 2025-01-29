package gcspec

type ID uint8

const null ID = 0

const (
	OK ID = iota + 1
	ERR
	REG
	VERIF
	REQ
	USRS
	RECIV
	CONN
	MSG
	DISCN
	DEREG
	SHTDWN
)

var codeToid map[byte]ID = map[byte]ID{
	0x01: OK,
	0x02: ERR,
	0x03: REG,
	0x04: VERIF,
	0x05: REQ,
	0x06: USRS,
	0x07: RECIV,
	0x08: CONN,
	0x09: MSG,
	0x0A: DISCN,
	0x0B: DEREG,
	0x0C: SHTDWN,
}

var idToCode map[ID]byte = map[ID]byte{
	OK:     0x01,
	ERR:    0x02,
	REG:    0x03,
	VERIF:  0x04,
	REQ:    0x05,
	USRS:   0x06,
	RECIV:  0x07,
	CONN:   0x08,
	MSG:    0x09,
	DISCN:  0x0A,
	DEREG:  0x0B,
	SHTDWN: 0x0C,
}

var errorCodes map[error]byte = map[error]byte{
	ErrorUndefined: 0x00,
	ErrorInvalid:   0x01,
	ErrorNotFound:  0x02,
	ErrorVersion:   0x03,
	ErrorHandshake: 0x04,
}
