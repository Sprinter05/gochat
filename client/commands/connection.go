package commands

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
)

/* STRUCTS */

// Specifies a message that is going through the connection
type Message struct {
	Sender    string    // Who is sending the message
	Content   string    // What the message contains
	Timestamp time.Time // When the message was sent
}

/* CONNECTION FUNCTIONS */

// Performs the socket connection to the server.
func SocketConnect(address string, port uint16, useTLS bool, noVerify bool) (con net.Conn, err error) {
	socket := net.JoinHostPort(address, strconv.FormatUint(uint64(port), 10))

	if useTLS {
		con, err = tls.Dial("tcp4", socket, &tls.Config{
			InsecureSkipVerify: noVerify,
		})
		if err != nil {
			return nil, err
		}

		return con, nil
	}

	// Default to non-TLS
	con, err = net.Dial("tcp4", socket)
	if err != nil {
		return nil, err
	}

	return con, nil
}

// Listens for a HELLO packet from the server when starting the connection,
// which determines that the client/server connection was started successfully.
func WaitConnect(data Command, server db.Server) error {
	cmd := new(spec.Command)

	conn := spec.Connection{
		Conn: data.Data.Conn,
		TLS:  server.TLS,
	}

	// Header listen
	hdErr := cmd.ListenHeader(conn)
	if hdErr != nil {
		return hdErr
	}

	// Header check
	chErr := cmd.HD.ClientCheck()
	if chErr != nil {
		data.Output("Incorrect header from server!", ERROR)
		return chErr
	}

	if data.Static.Verbose {
		data.Output(cmd.Contents(), PACKET)
	}

	// Payload listen
	pldErr := cmd.ListenPayload(conn)
	if pldErr != nil {
		return pldErr
	}

	if cmd.HD.Op != spec.HELLO {
		data.Output("invalid initial packet from the server", ERROR)
		return spec.ErrorUndefined
	}
	data.Output("succesfully connected to the server", RESULT)

	motd := string(cmd.Args[0])
	if motd == "" {
		return nil
	}

	str := fmt.Sprintf(
		"server MOTD (message of the day):\n%s",
		motd,
	)
	data.Output(str, INFO)

	return nil
}

/* LISTENING FUNCTIONS */

// Listens for incoming server packets. When a packet
// is received, it is stored in the packet waitlist
// A cleanup function that cleans up resources can be passed.
func ListenPackets(cmd Command, cleanup func()) {
	defer func() {
		if cmd.Data.Conn != nil {
			cmd.Data.Conn.Close()
		}

		cmd.Data.Conn = nil
		cmd.Data.User = nil

		cmd.Output("No longer listening for packets", INFO)
		cleanup()
	}()

	conn := spec.Connection{
		Conn: cmd.Data.Conn,
		TLS:  cmd.Data.Server.TLS,
	}

	for {
		if cmd.Data.Conn == nil {
			return
		}
		pct := spec.Command{}

		// Header listen
		hdErr := pct.ListenHeader(conn)
		if hdErr != nil {
			if cmd.Static.Verbose {
				cmd.Output(fmt.Sprintf("error in header listen: %s", hdErr), ERROR)
			}
			return
		}

		// Header check
		chErr := pct.HD.ClientCheck()
		if chErr != nil {
			if cmd.Static.Verbose {
				cmd.Output(fmt.Sprintf("incorrect header from server: %s", chErr), ERROR)
				cmd.Output(pct.Contents(), PACKET)
			}
			return
		}

		// Payload listen
		pldErr := pct.ListenPayload(conn)
		if pldErr != nil {
			if cmd.Static.Verbose {
				cmd.Output(fmt.Sprintf("error in payload listen: %s", pldErr), ERROR)
			}
			return
		}

		if cmd.Static.Verbose {
			cmd.Output("\r\033[K", COLOR)
			cmd.Output(fmt.Sprintf("The following packet has been received:\n%s", pct.Contents()), PACKET)
		}

		cmd.Data.Waitlist.Insert(pct)
	}
}
