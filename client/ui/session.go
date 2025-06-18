package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
)

/* SESSION */

// Subscribes to the default hooks of the server
// and updates the userlist once
func defaultSubscribe(t *TUI, s Server, output cmds.OutputFunc) {
	hooks := []string{"new_login", "new_logout"}
	data, _ := s.Online()

	for _, v := range hooks {
		ctx, cancel := timeout(s, data)
		defer data.Waitlist.Cancel(cancel)
		_, err := cmds.Sub(ctx, cmds.Command{
			Output: output,
			Static: &t.data,
			Data:   data,
		}, v)
		if err != nil {
			output(err.Error(), cmds.ERROR)
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

	ctx, cancel := timeout(s, cmd.Data)
	defer data.Waitlist.Cancel(cancel)
	_, err = cmds.Req(ctx, cmd, tab.name)
	if err != nil {
		ret := fmt.Errorf(
			"failed to request user data due to %s",
			err,
		)
		return ret
	}

	connected()
	return nil
}

/* NOTIFICATIONS */

type Notifications struct {
	data *models.Table[string, uint]
}

func (n Notifications) Notify(user string) {
	if n.data == nil {
		return
	}

	v, _ := n.data.Get(user)
	n.data.Add(user, v+1)
}

func (n Notifications) Users() []string {
	if n.data == nil {
		return make([]string, 0)
	}

	return n.data.Indexes()
}

func (n Notifications) Query(user string) uint {
	if n.data == nil {
		return 0
	}

	v, ok := n.data.Get(user)
	if !ok {
		return 0
	}

	return v
}

func (n Notifications) Zero(user string) {
	if n.data == nil {
		return
	}

	n.data.Add(user, 0)
}

func (n Notifications) Clear() {
	if n.data == nil {
		return
	}

	n.data.Clear()
}

// Renders the notification text for the current server
func (t *TUI) updateNotifications() {
	s := t.Active()
	curr := t.Buffer()
	notifs := s.Notifications()
	peding := notifs.Users()

	_, ok := s.Online()
	if !ok {
		t.area.bottom.ResizeItem(t.comp.notifs, 0, 0)
		return
	}

	var text strings.Builder
	for _, v := range peding {
		unread := notifs.Query(v)
		if unread == 0 {
			continue
		}

		if curr == v {
			notifs.Zero(curr)
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

	ctx, cancel := timeout(s, cmd.Data)
	defer cmd.Data.Waitlist.Cancel(cancel)
	_, err := cmds.Msg(ctx, cmd, tab.name, content)
	if err != nil {
		print("failed to send message: "+err.Error(), cmds.ERROR)
	}
}

// Waits for new messages to be sent to that user
func (t *TUI) receiveMessages(ctx context.Context, s Server) {
	defer func() {
		s.Buffers().Offline()
		s.Notifications().Clear()
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

		rCtx, cancel := timeout(s, data)
		msg, err := cmds.StoreMessage(
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

		s.Notifications().Notify(msg.Sender)
		t.updateNotifications()

		if msg.Sender == data.User.User.Username {
			print(ErrorMessageFromSelf.Error())
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

// Waits for new notifications of hooks from the server
func (t *TUI) receiveHooks(ctx context.Context, s Server) {
	defer func() {
		t.comp.users.SetText("")
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
