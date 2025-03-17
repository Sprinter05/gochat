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
// ! Deberia estar protegido con un mutex
var pendingBuffer map[uint16]uint8 = make(map[uint16]uint8, 4)

// Adds an id to the pending buffer
func AddPending(id uint16, op uint8) {
	pendingBuffer[id] = op
}

// Returns true if the packet is pending
func IsPending(id uint16) bool {
	_, ok := pendingBuffer[id]
	return ok
}

// Deletes an ID from the pending buffer
func acknoledgePending(id uint16) {
	delete(pendingBuffer, id)
}

func GetAllPending() map[uint16]uint8 {
	return pendingBuffer
}

func IsPendingEmpty() bool {
	return len(pendingBuffer) == 0
}

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
		processErr := GetServerCommand(pct.HD.Op)(&pct)
		if !(pct.HD.Op == spec.VERIF || pct.HD.Op == spec.RECIV) {
			pctReceived <- struct{}{}
		}
		if processErr != nil {
			fmt.Println(processErr)
		}
	}
}
