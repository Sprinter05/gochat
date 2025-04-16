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

var clientCmds = map[string]func(data CommandData, verbose *bool) error{
	"VER":     ver,
	"VERBOSE": verbose,
	"REQ":     req,
}

func FetchClientCmd(op string) func(data CommandData, verbose *bool) error {
	v, ok := clientCmds[op]
	if !ok {
		fmt.Printf("command %s not found\n", op)
		return nil
	}
	return v
}

func ver(data CommandData, verbose *bool) error {
	fmt.Printf("gochat version %d\n", spec.ProtocolVersion)
	return nil
}

func verbose(data CommandData, verbose *bool) error {
	*verbose = !*verbose
	if *verbose {
		fmt.Println("verbose mode on")
	} else {
		fmt.Println("verbose mode off")
	}
	return nil
}

func req(data CommandData, verbose *bool) error {
	pct, pctErr := spec.NewPacket(spec.REQ, 1, spec.EmptyInfo, data.Args...)
	if pctErr != nil {
		return pctErr
	}

	_, wErr := data.Con.Write(pct)
	return wErr
}
