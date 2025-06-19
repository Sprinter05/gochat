package commands

import (
	"context"
	"fmt"
	"slices"

	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
)

/* AUXILIARY FUNCTIONS */

// Requests the user logged in to get its permissions
func GetPermissions(ctx context.Context, cmd Command, uname string) (uint, error) {
	id := cmd.Data.NextID()
	packet, err := spec.NewPacket(
		spec.REQ,
		id,
		spec.EmptyInfo,
		[]byte(uname),
	)
	if err != nil {
		return 0, err
	}

	_, err = cmd.Data.Conn.Write(packet)
	if err != nil {
		return 0, err
	}

	verbosePrint("querying permissions...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.REQ, spec.ERR),
	)
	if err != nil {
		return 0, err
	}

	if reply.HD.Op == spec.ERR {
		return 0, spec.ErrorCodeToError(reply.HD.Info)
	}

	perms, err := spec.BytesToPermission(reply.Args[2])
	if err != nil {
		return 0, err
	}

	return perms, nil
}

// Performs the necessary operations to store a RECIV
// packet in the database (decryption, REQ (if necessary)
// insert...), then returns the decrypted message
func StoreMessage(ctx context.Context, reciv spec.Command, cmd Command) (Message, error) {
	src, err := db.GetUser(
		cmd.Static.DB,
		string(reciv.Args[0]),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if err != nil {
		// The user most likely has not been found, so a REQ is required
		_, reqErr := Req(ctx, cmd, string(reciv.Args[0]))
		if reqErr != nil {
			return Message{}, reqErr
		}
	}

	prvKey, pemErr := spec.PEMToPrivkey([]byte(cmd.Data.User.PrvKey))
	if pemErr != nil {
		return Message{}, pemErr
	}

	decrypted, decryptErr := spec.DecryptText(reciv.Args[2], prvKey)
	if decryptErr != nil {
		return Message{}, decryptErr
	}

	stamp, parseErr := spec.BytesToUnixStamp(reciv.Args[1])
	if parseErr != nil {
		return Message{}, parseErr
	}

	_, insertErr := db.StoreMessage(
		cmd.Static.DB,
		src.Username,
		cmd.Data.User.User.Username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
		string(decrypted),
		stamp,
	)
	if insertErr != nil {
		return Message{}, insertErr
	}

	return Message{
		Sender:    string(reciv.Args[0]),
		Content:   string(decrypted),
		Timestamp: stamp,
	}, nil
}

func tokenLogin(ctx context.Context, cmd Command, username string) error {
	id := cmd.Data.NextID()
	pct, err := spec.NewPacket(
		spec.LOGIN, id,
		spec.EmptyInfo,
		[]byte(username),
		[]byte(cmd.Data.Token),
	)
	if err != nil {
		return err
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	_, err = cmd.Data.Conn.Write(pct)
	if err != nil {
		return err
	}

	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return err
	}

	if reply.HD.Op == spec.ERR {
		cmd.Data.Token = ""
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	return nil
}

/* PRINTING FUNCTIONS */

// Prints out all local users on the current server and
// returns an array with its usernames.
func printServerLocalUsers(cmd Command) ([][]byte, error) {
	localUsers, err := db.GetServerLocalUsers(
		cmd.Static.DB,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)

	if err != nil {
		return [][]byte{}, err
	}

	users := make([][]byte, 0, len(localUsers))
	cmd.Output(fmt.Sprintf("local users from %s - %s:%d:",
		cmd.Data.Server.Name,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port),
		USRS,
	)

	for _, v := range localUsers {
		users = append(users, []byte(v.User.Username))
		cmd.Output(v.User.Username, USRS)
	}

	return users, nil
}

// Prints out all external users on the current server and
// returns an array with its usernames.
func printExternalUsers(cmd Command) ([][]byte, error) {
	externalUsers, err := db.GetRequestedUsers(cmd.Static.DB)

	if err != nil {
		return [][]byte{}, err
	}

	users := make([][]byte, 0, len(externalUsers))
	cmd.Output("all external users:", USRS)

	for _, v := range externalUsers {
		users = append(users, []byte(v.User.Username))
		cmd.Output(fmt.Sprintf("%s (%s - %s:%d)",
			v.User.Username,
			v.User.Server.Name,
			v.User.Server.Address,
			v.User.Server.Port),
			USRS,
		)
	}

	return users, nil
}

// Prints out all local users on the current server and
// returns an array with its usernames.
func printAllLocalUsers(cmd Command) ([][]byte, error) {
	localUsers, err := db.GetAllLocalUsers(
		cmd.Static.DB,
	)

	if err != nil {
		return [][]byte{}, err
	}

	users := make([][]byte, 0, len(localUsers))
	cmd.Output("all local users:", USRS)

	for _, v := range localUsers {
		str := fmt.Sprintf(
			"%s (%s - %s:%d)",
			v.User.Username,
			v.User.Server.Name,
			v.User.Server.Address,
			v.User.Server.Port,
		)
		users = append(users, []byte(str))
		cmd.Output(str, USRS)
	}

	return users, nil
}

// Prints a packet.
func packetPrint(pct []byte, cmd Command) {
	pctCmd := spec.ParsePacket(pct)
	str := fmt.Sprintf(
		"Client packet to be sent:\n%s",
		pctCmd.Contents(),
	)
	cmd.Output(str, PACKET)
}

// Prints text if the verbose mode is on.
func verbosePrint(text string, args Command) {
	if args.Static.Verbose {
		args.Output(text, INTERMEDIATE)
	}
}

/* WAITLIST FUNCTIONS */

// Returns a function that returns true if the received command fulfills
// the given conditions in the arguments (ID and operations).
// This is used to dinamically create functions that retrieve commands
// from the waitlist with waitlist.Get()
func Find(id spec.ID, ops ...spec.Action) func(cmd spec.Command) bool {
	return func(cmd spec.Command) bool {
		if cmd.HD.ID == id && slices.Contains(ops, cmd.HD.Op) {
			return true
		}

		return false
	}
}

// Returns an appropiate waitlist
func DefaultWaitlist() models.Waitlist[spec.Command] {
	return models.NewWaitlist(0, func(a spec.Command, b spec.Command) int {
		switch {
		case a.HD.ID > b.HD.ID:
			return 1
		case a.HD.ID < b.HD.ID:
			return -1
		default:
			return 0
		}
	})
}
