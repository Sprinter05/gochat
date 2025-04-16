package main

import (
	"fmt"
	"net"

	"github.com/Sprinter05/gochat/internal/spec"
)

type CommandData struct {
	Args [][]byte
	Con  net.Conn
	// TODO: DB...
}

var clientCmds = map[string]func(data CommandData) error{
	"VER": ver,
	"REQ": req,
}

func ver(data CommandData) error {
	fmt.Printf("gochat version %d\n", spec.ProtocolVersion)
	return nil
}

func req(data CommandData) error {
	pct, pctErr := spec.NewPacket(spec.REQ, 1, spec.EmptyInfo, data.Args...)
	if pctErr != nil {
		return pctErr
	}

	_, wErr := data.Con.Write(pct)
	return wErr
}
