package ui

import (
	"context"
	"fmt"
	"strings"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/gdamore/tcell/v2"
)

type Command struct {
	Operation string
	Arguments []string

	serv  Server
	print func(string, cmds.OutputType)
}

type operation struct {
	fun    func(*TUI, Command)
	nArgs  uint
	format string
}

var commands map[string]operation = map[string]operation{
	"buffers": {
		fun:    listBuffers,
		nArgs:  0,
		format: "/buffers",
	},
	"connect": {
		fun:    connectServer,
		nArgs:  0,
		format: "/connect (-noverify)",
	},
	"register": {
		fun:    registerUser,
		nArgs:  1,
		format: "/register <username>",
	},
	"users": {
		fun:    listUsers,
		nArgs:  1,
		format: "/users <online/all/local>",
	},
	"login": {
		fun:    loginUser,
		nArgs:  1,
		format: "/login <user>",
	},
	"logout": {
		fun:    logoutUser,
		nArgs:  0,
		format: "/logout",
	},
	"disconnect": {
		fun:    disconnectServer,
		nArgs:  0,
		format: "/disconnect",
	},
	"version": {
		fun:    showVersion,
		nArgs:  0,
		format: "/version",
	},
	"tls": {
		fun:    toggleTLS,
		nArgs:  1,
		format: "/tls <on/off>",
	},
	"request": {
		fun:    userRequest,
		nArgs:  0,
		format: "/request",
	},
	"clear": {
		fun:    clearSystem,
		nArgs:  0,
		format: "/clear",
	},
}

func (t *TUI) parseCommand(text string) {
	lower := strings.ToLower(text)
	parts := strings.Split(lower, " ")

	if parts[0] == "" {
		t.showError(ErrorEmptyCmd)
		return
	}

	t.history.Add(lower)

	cmd := Command{
		Operation: parts[0],
		Arguments: parts[1:],
		serv:      t.Active(),
		print:     t.systemMessage(parts[0]),
	}

	op, ok := commands[cmd.Operation]
	if !ok {
		t.showError(ErrorInvalidCmd)
		return
	}

	if len(cmd.Arguments) < int(op.nArgs) {
		var builder strings.Builder
		parts := strings.Split(op.format, " ")
		builder.WriteString("[yellow]" + parts[0] + "[-]")
		for _, v := range parts[1:] {
			builder.WriteString(" [green]" + v + "[-]")
		}

		str := fmt.Sprintf("%s: %s", ErrorArguments, builder.String())
		cmd.print(str, cmds.RESULT)
		return
	}

	go op.fun(t, cmd)
}

// AUX

func (c Command) createCmd(t *TUI, d *cmds.Data) (cmds.Command, [][]byte) {
	array := make([][]byte, len(c.Arguments))
	for i, v := range c.Arguments {
		array[i] = []byte(v)
	}

	return cmds.Command{
		Data:   d,
		Static: &t.data,
		Output: c.print,
	}, array
}

// COMMANDS

func clearSystem(t *TUI, cmd Command) {
	buf := cmd.serv.Buffers().current
	tab, ok := cmd.serv.Buffers().tabs.Get(buf)
	if !ok {
		panic("missing current buffer")
	}

	count := 0
	msgs := tab.messages.Copy(0)
	for _, v := range msgs {
		if v.Sender == "System" {
			tab.messages.Remove(v)
			count += 1
		}
	}

	if count > 0 {
		t.renderBuffer(buf)
		cmd.print(fmt.Sprintf(
			"cleared %d system messages!",
			count,
		), cmds.RESULT)
	}
}

func userRequest(t *TUI, cmd Command) {
	buf := cmd.serv.Buffers().current
	data, _ := cmd.serv.Online()
	tab, exists := cmd.serv.Buffers().tabs.Get(buf)

	if data == nil {
		cmd.print("cannot request on a local server!", cmds.ERROR)
		return
	}

	if exists && tab.system {
		cmd.print("cannot request on a system buffer!", cmds.ERROR)
		return
	}

	err := t.requestUser(cmd.serv, buf, cmd.print)
	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
	}
}

func toggleTLS(t *TUI, cmd Command) {
	data, _ := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv)
	defer c.Data.Waitlist.Cancel(cancel)
	r := cmds.TLS(ctx, c, args...)

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}

	i := t.comp.servers.GetCurrentItem()
	addr := cmd.serv.Source()
	if cmd.Arguments[0] == "on" {
		t.comp.servers.SetItemText(
			i, cmd.serv.Name(),
			addr.String()+" (TLS)",
		)
		cmd.print("TLS is now enabled", cmds.RESULT)
	} else { // off
		t.comp.servers.SetItemText(
			i, cmd.serv.Name(),
			addr.String(),
		)
		cmd.print("TLS is now disabled", cmds.RESULT)
	}
}

func showVersion(t *TUI, cmd Command) {
	str := fmt.Sprintf(
		"\n* Client TUI version: [orange::i]v%.1f[-::-]\n* Protocol version: [orange::i]v%d[-::-]",
		tuiVersion,
		spec.ProtocolVersion,
	)
	cmd.print(str, cmds.RESULT)
}

func disconnectServer(t *TUI, cmd Command) {
	data, _ := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv)
	defer c.Data.Waitlist.Cancel(cancel)
	r := cmds.Discn(ctx, c, args...)

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}

	t.comp.input.SetLabel(defaultLabel)
	t.comp.servers.SetSelectedTextColor(tcell.ColorPurple)
}

func logoutUser(t *TUI, cmd Command) {
	data, _ := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv)
	defer c.Data.Waitlist.Cancel(cancel)
	r := cmds.Logout(ctx, c, args...)

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}

	data.Waitlist.Cancel(data.Logout)
	t.comp.input.SetLabel(defaultLabel)
	cleanupSession(t, cmd.serv)
}

func loginUser(t *TUI, cmd Command) {
	data, ok := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	if data.IsLoggedIn() {
		cmd.print(ErrorLoggedIn.Error(), cmds.ERROR)
		return
	}

	if !ok {
		cmd.print(ErrorOffline.Error(), cmds.ERROR)
		return
	}

	pswd, err := newLoginPopup(t, "Enter the account's password...")
	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	lCtx, lCancel := timeout(cmd.serv)
	defer c.Data.Waitlist.Cancel(lCancel)
	r := cmds.Login(lCtx, c, args[0], []byte(pswd))

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}

	uname := data.User.User.Username
	t.comp.input.SetLabel(unameLabel(uname))

	ctx, cancel := context.WithCancel(cmd.serv.Connection().Get())
	data.Logout = cancel
	go t.receiveMessages(ctx, cmd.serv)

	cmd.print("recovering messages...", cmds.INTERMEDIATE)
	rCtx, rCancel := timeout(cmd.serv)
	defer c.Data.Waitlist.Cancel(rCancel)
	reciv := cmds.Reciv(rCtx, c)
	if reciv.Error != nil {
		if reciv.Error == spec.ErrorEmpty {
			cmd.print("no new messages have been received", cmds.RESULT)
		} else {
			cmd.print(reciv.Error.Error(), cmds.ERROR)
			return
		}
	}
}

func listUsers(t *TUI, cmd Command) {
	data, _ := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv)
	defer c.Data.Waitlist.Cancel(cancel)
	r := cmds.Usrs(ctx, c, args...)

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}

	var list strings.Builder
	list.WriteString("Showing " + cmd.Arguments[0] + " users:\n")
	if len(r.Arguments) == 0 {
		list.WriteString("No users to be shown.\n")
	}

	for _, v := range r.Arguments {
		list.WriteString("- [pink::i]" + string(v) + "[-::-]\n")
	}

	l := list.Len()
	cmd.print(list.String()[:l-1], cmds.RESULT)
}

func registerUser(t *TUI, cmd Command) {
	data, ok := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	if !ok {
		cmd.print(ErrorOffline.Error(), cmds.ERROR)
		return
	}

	pswd, err := newLoginPopup(t, "Enter a password...")
	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	check, err := newLoginPopup(t, "Repeat your password...")
	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	if pswd != check {
		cmd.print("passwords do not match!", cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv)
	defer c.Data.Waitlist.Cancel(cancel)
	r := cmds.Reg(ctx, c, args[0], []byte(pswd))

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}
}

func connectServer(t *TUI, cmd Command) {
	addr := cmd.serv.Source()
	if addr == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	parts := strings.Split(addr.String(), ":")
	data, ok := cmd.serv.Online()
	if ok {
		cmd.print(ErrorAlreadyOnline.Error(), cmds.ERROR)
		return
	}

	args := make([][]byte, 0)
	args = append(args, []byte(parts[0]))
	args = append(args, []byte(parts[1]))

	if len(cmd.Arguments) >= 1 {
		args = append(args, []byte(cmd.Arguments[0]))
	}

	cmd.print("attempting to connect...", cmds.INTERMEDIATE)
	c, _ := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv)
	defer c.Data.Waitlist.Cancel(cancel)
	r := cmds.Conn(ctx, c, args...)

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}

	cmd.serv.Connection().Set(context.Background())
	t.comp.servers.SetSelectedTextColor(tcell.ColorGreen)

	c.Output = t.systemMessage("", defaultBuffer)
	go cmds.Listen(c, func() {
		cmd.serv.Buffers().Offline()
		c.Data.Waitlist.Cancel(data.Logout)
		c.Data.Waitlist.Cancel(cmd.serv.Connection().Cancel)

		t.comp.input.SetLabel(defaultLabel)
		t.comp.servers.SetSelectedTextColor(tcell.ColorPurple)

		cleanupSession(t, cmd.serv)
		t.notifs.Clear()

		discn := t.systemMessage()
		discn("You are no longer connected to this server!", cmds.INFO)
	})
}

func listBuffers(t *TUI, cmd Command) {
	var list strings.Builder
	bufs := cmd.serv.Buffers()
	l := bufs.tabs.GetAll()

	list.WriteString("showing active server buffers: ")
	for i, v := range l {
		hidden := ""
		if v.index == -1 {
			hidden = " - [gray::i]Hidden[-::-]"
		}

		str := fmt.Sprintf(
			"\n[green]%d:[-::-] %s%s",
			i+1, v.name, hidden,
		)

		list.WriteString(str)
	}

	content := list.String()

	cmd.print(content, cmds.RESULT)
}
