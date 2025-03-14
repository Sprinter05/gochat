package main

// Manages the client listener

import (
	"bufio"
	"context"
	"fmt"
	"net"

	"github.com/Sprinter05/gochat/internal/spec"
)

// Buffer where the pending packet's ID is allocated as a key with the operation code as its value
// ! No deberia ser global
// ! Deberia estar protegido con un mutex
// ! En make() dale un segundo argumento con el tamaño para prealocarlo, si no cada vez q añadas un paquete añades delay al realocarlo
var PendingBuffer map[uint16]uint8 = make(map[uint16]uint8)

// Starts listening for packets
func Listen(con net.Conn, ctx context.Context, pctReceived chan struct{}) {

	cl := spec.Connection{
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
}
