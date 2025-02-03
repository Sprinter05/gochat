package main

import (
	"log"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// FUNCTIONS

func registerUser(u *User, cmd gc.Command) {
	// Assign parameters to the user
	u.name = username(cmd.Args[0])

	// Make reply packets
	ret := gc.ErrorCode(gc.ErrorHandshake)
	errpak, _ := gc.NewPacket(gc.ERR, ret, nil)
	// TODO: Create random string to decypher
	vpak, _ := gc.NewPacket(gc.VERIF, gc.EmptyInfo, nil)

	// Assign public key
	key, err := pemToPub(cmd.Args[1])
	if err != nil {
		// Invalid public key given
		log.Print(err)
		u.conn.Write(errpak)
		return
	}
	u.pubkey = key
	u.conn.Write(vpak)
}
