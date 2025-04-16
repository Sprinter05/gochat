package main

import (
	"fmt"

	"github.com/Sprinter05/gochat/internal/spec"
)

var clientCmds = map[string]func(data *ShellData, args [][]byte) error{
	"VER":     ver,
	"VERBOSE": verbose,
	"REQ":     req,
}

func FetchClientCmd(op string) func(data *ShellData, args [][]byte) error {
	v, ok := clientCmds[op]
	if !ok {
		fmt.Printf("command %s not found\n", op)
		return nil
	}
	return v
}

func ver(data *ShellData, args [][]byte) error {
	fmt.Printf("gochat version %d\n", spec.ProtocolVersion)
	return nil
}

func verbose(data *ShellData, args [][]byte) error {
	data.Verbose = !data.Verbose
	if data.Verbose {
		fmt.Println("verbose mode on")
	} else {
		fmt.Println("verbose mode off")
	}
	return nil
}

func req(data *ShellData, args [][]byte) error {
	pct, pctErr := spec.NewPacket(spec.REQ, 1, spec.EmptyInfo, args...)
	if pctErr != nil {
		return pctErr
	}

	if data.Verbose {
		fmt.Println("The following packet is about to be sent:")
		cmd := spec.ParsePacket(pct)
		cmd.Print()
	}

	_, wErr := data.ClientCon.Conn.Write(pct)
	return wErr
}
