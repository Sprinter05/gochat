package main

import (
	"log"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// FUNCTIONS

func registerUser(h *Hub, u *User, cmd gc.Command) {
	// Assign parameters to the user
	u.name = username(cmd.Args[0])

	// Error reply packer
	ret := gc.ErrorCode(gc.ErrorHandshake)
	errpak, _ := gc.NewPacket(gc.ERR, ret, nil)

	// Assign public key
	key, err := gc.PemToPubkey(cmd.Args[1])
	if err != nil {
		//* Error with public key
		log.Print(err)
		u.conn.Write(errpak)
		return
	}
	u.pubkey = key

	// Create random cypher
	enc, err := gc.RandomCypher(u.pubkey)
	if err != nil {
		//* Error with cyphering
		log.Print(err)
		u.conn.Write(errpak)
		return
	}
	vpak, _ := gc.NewPacket(gc.VERIF, gc.EmptyInfo, []gc.Arg{gc.Arg(enc)})
	u.conn.Write(vpak)

	// Add user to the hub
	ip := ip(u.conn.RemoteAddr().String())
	h.mut.Lock()
	h.users[ip] = u
	h.mut.Unlock()
}
