package main

// Protocol version
const Version uint8 = 1
const HeaderSize = 2 // bytes

// Header field sizes
const HeaderVersionBits = 3
const HeaderActionBits = 7
const HeaderInfoBits = 6

// Map of all server action codes
var serverActionCodes map[uint8]string = map[uint8]string{
	0x01: "OK",
	0x02: "ERR",
	0x03: "SHTDWN",
	0x04: "VERIF",
	0x05: "REQ",
	0x06: "USRS",
	0x07: "RECIV",
}

// Map of all client action codes
var clientActionCodes map[uint8]string = map[uint8]string{
	0x01: "OK",
	0x02: "ERR",
	0x03: "REG",
	0x04: "VERIF",
	0x05: "REQ",
	0x06: "USRS",
	0x07: "CONN",
	0x08: "MSG",
	0x09: "DISCN",
	0x0A: "DEREG",
}

func GetServerActionCode(i uint8) string {
	v, ok := serverActionCodes[i]
	if !ok {
		return ""
	}
	return v
}

func GetClientActionCode(i uint8) string {
	v, ok := clientActionCodes[i]
	if !ok {
		return ""
	}
	return v
}
