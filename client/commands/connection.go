package commands

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/Sprinter05/gochat/internal/spec"
)

type Message struct {
	Sender    string
	Content   string
	Timestamp time.Time
}

// Connects to the gochat server given its address and port
func Connect(address string, port uint16, useTLS bool, noVerify bool) (con net.Conn, err error) {
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

// Listens for incoming server packets. When a packet
// is received, it is stored in the packet waitlist
// A cleanup function that cleans up resources can be passed
func Listen(cmd Command, cleanup func()) {
	defer func() {
		if cmd.Data.Conn != nil {
			cmd.Data.Conn.Close()
		}

		cmd.Data.Conn = nil
		cmd.Data.User = nil

		<-time.After(50 * time.Millisecond)
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
			cmd.Output(fmt.Sprintf("\r\033[KThe following packet has been received:\n%s", pct.Contents()), PACKET)
		}

		cmd.Data.Waitlist.Insert(pct)
	}
}

// Listens for an OK packet from the server when starting the connection,
// which determines that the client/server was started successfully
func ConnectionStart(data Command) error {
	cmd := new(spec.Command)

	conn := spec.Connection{
		Conn: data.Data.Conn,
		TLS:  data.Data.Server.TLS,
	}

	// Header listen
	hdErr := cmd.ListenHeader(conn)
	if hdErr != nil {
		return hdErr
	}

	// Header check
	chErr := cmd.HD.ClientCheck()
	if chErr != nil {
		if data.Static.Verbose {
			str := fmt.Sprintf(
				"Incorrect header from server:\n%s",
				cmd.Contents(),
			)
			data.Output(str, PACKET)
		}
		return chErr
	}

	// Payload listen
	pldErr := cmd.ListenPayload(conn)
	if pldErr != nil {
		return pldErr
	}

	if cmd.HD.Op == 1 {
		data.Output("successfully connected to the server", RESULT)
	} else {
		return spec.ErrorUndefined
	}

	return nil
}

/*
// Receives a slice of command operations to listen to, then starts
// listening until a received packet fits one of the actions provided
// and returns it
func ListenResponse(data Command, id spec.ID, ops ...spec.Action) (spec.Command, error) {
	//  timeouts
	var cmd spec.Command

	for !(slices.Contains(ops, cmd.HD.Op)) {
		cmd = spec.Command{}
		// Header listen
		hdErr := cmd.ListenHeader(data.Data.ClientCon)
		if hdErr != nil {
			return cmd, hdErr
		}

		// Header check
		chErr := cmd.HD.ClientCheck()
		if chErr != nil {
			if data.Static.Verbose {
				str := fmt.Sprintf(
					"Incorrect header from server:\n%s",
					cmd.Contents(),
				)
				data.Output(str, PACKET)
			}
			return cmd, chErr
		}

		// Payload listen
		pldErr := cmd.ListenPayload(data.Data.ClientCon)
		if pldErr != nil {
			return cmd, pldErr
		}
	}

	if data.Static.Verbose {
		str := fmt.Sprintf(
			"Packet received from server:\n%s",
			cmd.Contents(),
		)
		data.Output(str, PACKET)
	}

	if cmd.HD.ID != id {
		return cmd, fmt.Errorf("unexpected ID received")
	}
	return cmd, nil
}
*/
