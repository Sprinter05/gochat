package main

// Manages the client listener

import (
	"bufio"
	"io"
	"log"
	"net"

	"github.com/Sprinter05/gochat/gcspec"
)

// Buffer where packets whose response is pending are allocated
var PacketBuffer map[uint16]*[]byte = make(map[uint16]*[]byte)

// Starts listening for packets
func Listen(con net.Conn) {

	cl := &gcspec.Connection{
		Conn: con,
		RD:   bufio.NewReader(con),
	}

	defer cl.Conn.Close()

	for {
		pct := spec.Command{}
		headerErr := pct.ListenHeader(cl)
		if headerErr != nil {
			fmt.Println("Error in header listen:")
			fmt.Println(headerErr.Error())
			pct.Print()
		}
		payloadErr := pct.ListenPayload(cl)
		if payloadErr != nil {
			fmt.Println("Error in payload listen:")
			fmt.Println(payloadErr.Error())
			pct.Print()
		}
		// If the server packet was correct, by this point in the code, it has been completely received

		if IsVerbose {
			ClearPrompt()
			pct.ShellPrint()
			pct.Print()
		}
		// The packet is processed and the proper action is performed
		processErr := ServerCmds[pct.HD.Op](&pct)
		if !(pct.HD.Op == spec.VERIF || pct.HD.Op == spec.RECIV) {
			pctReceived <- struct{}{}
		}
		if processErr != nil {
			fmt.Println(processErr)
	}
}
