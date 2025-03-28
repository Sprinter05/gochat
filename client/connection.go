package main

// Manages the client listener

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"net"

	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
)

/*
// Buffer where the pending packet's ID is allocated as a key with the operation code as its value
// ! Deberia estar protegido con un mutex
var pendingBuffer map[uint16]uint8 = make(map[uint16]uint8, 4)
*/

var pendingBuffer = models.NewTable[uint16, uint8](4)

func GetMaxID(init uint16) uint16 {
	// Defines anonimously a function to obtain the maximum ID in the buffer
	return pendingBuffer.Apply(func(a, b uint16) uint16 {
		if a > b {
			return a
		}
		return b
	}, init)
}

func GetAllPending() map[uint16]uint8 {
	return pendingBuffer.GetData()
}

func AcknoledgePending(id uint16) {
	pendingBuffer.Remove(id)
}

func AddPending(id uint16, op uint8) {
	pendingBuffer.Add(id, op)
}

func IsPendingEmpty() bool {
	return pendingBuffer.GetAll() == nil
}

func IsPending(id uint16) bool {
	_, isPending := pendingBuffer.Get(id)
	return isPending
}

// Starts listening for packets
func Listen(con net.Conn, ctx context.Context, pctReceived chan struct{}, db *sql.DB) {

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
		processErr := GetServerCommand(pct.HD.Op).Run(pct, db)
		if !(pct.HD.Op == spec.VERIF || pct.HD.Op == spec.RECIV) {
			pctReceived <- struct{}{}
		}
		if processErr != nil {
			fmt.Println(processErr)
		}
	}
}
