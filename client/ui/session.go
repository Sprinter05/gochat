package ui

import (
	"context"
	"fmt"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
)

/* SESSION */

// Returns to default buffer and delets all others
func cleanupSession(t *TUI, s Server) {
	i, _ := t.findBuffer(defaultBuffer)
	t.changeBuffer(i)

	bufs := s.Buffers()

	for _, v := range bufs.GetAll() {
		if v == defaultBuffer {
			continue
		}

		i, ok := t.findBuffer(v)
		t.removeBuffer(v)
		if ok {
			t.comp.buffers.RemoveItem(i)
		}
	}
}

/* USERS */

// Requests a user's key on buffer connection
func (t *TUI) requestUser(s Server, name string, output func(string, cmds.OutputType)) {

	tab, exists := s.Buffers().tabs.Get(name)
	data, ok := s.Online()

	connected := func() {
		tab.connected = true
		if tab.messages.Len() == 0 {
			getOldMessages(t, s, name)
			print("This is the start of this conversation with "+name, cmds.INFO)
		}
	}

	if exists && tab.system {
		return
	}

	if data == nil {
		return
	}

	if !ok || !exists {
		output("to start messaging a user, please connect and login first!", cmds.ERROR)
		return
	}

	ok, err := db.ExternalUserExists(t.data.DB, name)
	if err != nil {
		output(err.Error(), cmds.ERROR)
		return
	}

	if ok && !tab.connected {
		connected()
		return
	}

	// output("attempting to get user data...", cmds.INTERMEDIATE)

	cmd := cmds.Command{
		Output: output,
		Static: &t.data,
		Data:   data,
	}

	ctx, cancel := timeout(s)
	defer data.Waitlist.Cancel(cancel)
	r := cmds.Req(ctx, cmd, []byte(tab.name))
	if r.Error != nil {
		str := fmt.Sprintf(
			"failed to request user data due to %s!",
			r.Error,
		)
		output(str, cmds.ERROR)
		return
	}

	connected()
}

/* MESSAGES */

// Sends a message to the remote connection if possible
func (t *TUI) remoteMessage(content string) {
	print := t.systemMessage("message")

	s := t.Active()
	tab := s.Buffers().Current()

	data, ok := s.Online()

	if tab != nil && tab.system {
		return
	}

	if data == nil {
		return
	}

	if tab == nil || !ok || !tab.connected {
		print("failed to send message: "+ErrorNoRemoteUser.Error(), cmds.ERROR)
		return
	}

	cmd := cmds.Command{
		Output: func(text string, outputType cmds.OutputType) {},
		Static: &t.data,
		Data:   data,
	}

	ctx, cancel := timeout(s)
	defer cmd.Data.Waitlist.Cancel(cancel)
	r := cmds.Msg(ctx, cmd, []byte(tab.name), []byte(content))
	if r.Error != nil {
		print("failed to send message: "+r.Error.Error(), cmds.ERROR)
	}
}

// Waits for new messages to be sent to that user
func (t *TUI) receiveMessages(ctx context.Context, s Server) {
	defer s.Buffers().Offline()
	data, _ := s.Online()
	output := t.systemMessage("reciv", defaultBuffer)

	print := func(msg string) {
		if t.data.Verbose {
			<-time.After(50 * time.Millisecond)
			output(msg, cmds.ERROR)
		}
	}

	for {
		cmd, err := data.Waitlist.Get(
			ctx,
			cmds.Find(spec.NullID, spec.RECIV),
		)
		if err != nil {
			print(err.Error())
			return
		}

		if !data.IsUserLoggedIn() {
			print("not logged in, ignoring incoming reciv")
			continue
		}

		rCtx, cancel := timeout(s)
		msg, err := cmds.StoreReciv(
			rCtx, cmd,
			cmds.Command{
				Output: func(text string, outputType cmds.OutputType) {},
				Static: &t.data,
				Data:   data,
			},
		)
		data.Waitlist.Cancel(cancel)

		if err != nil {
			print(err.Error())
			continue
		}

		t.SendMessage(Message{
			Buffer:    msg.Sender,
			Sender:    msg.Sender,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
			Source:    s.Source(),
		})
	}

}
