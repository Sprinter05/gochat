package main

import (
	"fmt"
	"log"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// FUNCTIONS

func registerUser(h *Hub, u *User, cmd gc.Command) {
	// TODO: Check if user is already logged in or in database
	// Assign parameters to the user
	u.name = username(cmd.Args[0])

	// Error reply packer
	ret := gc.ErrorCode(gc.ErrorHandshake)
	errpak, _ := gc.NewPacket(gc.ERR, ret, nil)

	// Assign public key
	key, err := gc.PEMToPubkey(cmd.Args[1])
	if err != nil {
		//* Error with public key
		log.Println(err)
		u.conn.Write(errpak)
		return
	}
	u.pubkey = key

	// Create random cypher
	ran := randText()
	fmt.Println(string(ran)) //! TEST
	enc, err := gc.EncryptText(ran, key)
	if err != nil {
		//* Error with cyphering
		log.Println(err)
		u.conn.Write(errpak)
		return
	}
	vpak, _ := gc.NewPacket(gc.VERIF, gc.EmptyInfo, []gc.Arg{gc.Arg(enc)})
	u.conn.Write(vpak)

	//TODO: Change so that it has to be in an unverified list until the decyphered payload is sent
	ip := ip(u.conn.RemoteAddr().String())
	h.mut.Lock()
	h.users[ip] = u
	h.mut.Unlock()
}
