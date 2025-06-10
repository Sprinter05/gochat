package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
)

/* SESSION */

// Subscribes to the default hooks of the server
// and updates the userlist once
func defaultSubscribe(t *TUI, s Server, output cmds.OutputFunc) {
	hooks := []string{"new_login", "new_logout"}
	data, _ := s.Online()

	for _, v := range hooks {
		ctx, cancel := timeout(s)
		defer data.Waitlist.Cancel(cancel)
		reply := cmds.Sub(ctx, cmds.Command{
			Output: output,
			Static: &t.data,
			Data:   data,
		}, v)
		if reply.Error != nil {
			output(reply.Error.Error(), cmds.ERROR)
			continue
		}
	}

	empty := func(string, cmds.OutputType) {}
	updateOnlineUsers(t, s, empty)
}

// Returns to default buffer and delets all others.
// Also hides notifications
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

	t.comp.notifs.SetText("")
}

/* USERS */

// Requests a user's key on buffer connection
func (t *TUI) requestUser(s Server, name string, output cmds.OutputFunc) error {
	tab, exists := s.Buffers().tabs.Get(name)
	data, ok := s.Online()

	connected := func() {
		if !tab.connected {
			getOldMessages(t, s, name)
		}
		tab.connected = true
	}

	if exists && tab.system {
		return nil
	}

	if data == nil {
		return nil
	}

	if !ok || !exists {
		return ErrorNotLoggedIn
	}

	ok, err := db.ExternalUserExists(t.data.DB, name, data.Server.Address, data.Server.Port)
	if err != nil {
		return err
	}

	if ok {
		if !tab.connected {
			connected()
		}
		return nil
	}

	// output("attempting to get user data...", cmds.INTERMEDIATE)

	cmd := cmds.Command{
		Output: output,
		Static: &t.data,
		Data:   data,
	}

	ctx, cancel := timeout(s)
	defer data.Waitlist.Cancel(cancel)
	r := cmds.Req(ctx, cmd, tab.name)
	if r.Error != nil {
		ret := fmt.Errorf(
			"failed to request user data due to %s",
			r.Error,
		)
		return ret
	}

	connected()
	return nil
}

/* NOTIFICATIONS */

// Renders the notification text for the current server
func (t *TUI) updateNotifications() {
	s := t.Active()
	curr := t.Buffer()
	peding := t.notifs.Indexes()

	_, ok := s.Online()
	if !ok {
		t.area.bottom.ResizeItem(t.comp.notifs, 0, 0)
		return
	}

	var text strings.Builder
	for _, v := range peding {
		unread, _ := t.notifs.Get(v)
		if unread == 0 {
			continue
		}

		if curr == v {
			t.notifs.Add(curr, 0)
			continue
		}

		str := fmt.Sprintf(
			"[cyan::b]%s[-:-:-]: [green]%d[-] | ",
			v, unread,
		)
		text.WriteString(str)
	}

	if text.String() == "" {
		t.comp.notifs.SetText("\n No notifications")
		t.area.bottom.ResizeItem(t.comp.notifs, 0, 0)
		return
	}

	l := text.Len()
	t.comp.notifs.SetText("\n " + text.String()[:l-3])
	t.area.bottom.ResizeItem(t.comp.notifs, notifSize, 0)
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
	r := cmds.Msg(ctx, cmd, tab.name, content)
	if r.Error != nil {
		print("failed to send message: "+r.Error.Error(), cmds.ERROR)
	}
}

// Waits for new messages to be sent to that user
func (t *TUI) receiveMessages(ctx context.Context, s Server) {
	defer func() {
		s.Buffers().Offline()
		t.notifs.Clear()
	}()

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

		if !data.IsLoggedIn() {
			print("not logged in, ignoring incoming reciv")
			continue
		}

		rCtx, cancel := timeout(s)
		msg, err := cmds.StoreReciv(
			rCtx, cmd,
			cmds.Command{
				Output: func(string, cmds.OutputType) {},
				Static: &t.data,
				Data:   data,
			},
		)
		data.Waitlist.Cancel(cancel)

		if err != nil {
			print(err.Error())
			continue
		}

		v, _ := t.notifs.Get(msg.Sender)
		t.notifs.Add(msg.Sender, v+1)
		t.updateNotifications()

		t.SendMessage(Message{
			Buffer:    msg.Sender,
			Sender:    msg.Sender,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
			Source:    s.Source(),
		})
	}
}

// Waits for new notifications of hooks from the server
func (t *TUI) receiveHooks(ctx context.Context, s Server) {
	defer func() {
		t.comp.users.SetText("", false)
	}()

	data, _ := s.Online()
	output := t.systemMessage("hook", defaultBuffer)

	print := func(msg string) {
		if t.data.Verbose {
			<-time.After(50 * time.Millisecond)
			output(msg, cmds.ERROR)
		}
	}
	empty := func(string, cmds.OutputType) {}

	for {
		cmd, err := data.Waitlist.Get(
			ctx,
			cmds.Find(spec.NullID, spec.HOOK),
		)
		if err != nil {
			print(err.Error())
			return
		}

		if !data.IsLoggedIn() {
			print("not logged in, ignoring incoming hook")
			continue
		}

		hook := spec.Hook(cmd.HD.Info)

		switch hook {
		case spec.HookPermsChange:
			output(
				"Your permission level in the server has changed!",
				cmds.INFO,
			)
		case spec.HookDuplicateSession:
			output(
				"Someone has tried to log in with your account from a different endpoint!",
				cmds.INFO,
			)
		case spec.HookNewLogin, spec.HookNewLogout:
			if t.Active().Name() == s.Name() {
				updateOnlineUsers(t, s, empty)
			}
		}
	}
}
