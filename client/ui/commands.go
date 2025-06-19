package ui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/gdamore/tcell/v2"
)

type Command struct {
	Operation string
	Arguments []string

	serv  Server
	print cmds.OutputFunc
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
		nArgs:  2,
		format: "/users <remote/local> <all/online/server> (-perms)",
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
	"import": {
		fun:    importKey,
		nArgs:  2,
		format: "/import <username> <path>",
	},
	"export": {
		fun:    exportKey,
		nArgs:  1,
		format: "/export <username>",
	},
	"subscribe": {
		fun:    subEvent,
		nArgs:  1,
		format: "/subscribe <hook>",
	},
	"unsubscribe": {
		fun:    unsubEvent,
		nArgs:  1,
		format: "/unsubscribe <hook>",
	},
	"deregister": {
		fun:    deregisterUser,
		nArgs:  1,
		format: "/deregister <user>",
	},
	"admin": {
		fun:    adminOperation,
		nArgs:  1,
		format: "/admin <operation> <arg_1> <arg_2> ... <arg_n>",
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

func (c Command) createCmd(t *TUI, d *cmds.Data) (cmds.Command, []string) {
	return cmds.Command{
		Data:   d,
		Static: &t.data,
		Output: c.print,
	}, c.Arguments
}

func askForNewPassword(t *TUI) (string, error) {
	pswd, err := newLoginPopup(t, "Enter a password...")
	if err != nil {
		return "", err
	}

	check, err := newLoginPopup(t, "Repeat your password...")
	if err != nil {
		return "", err
	}

	if pswd != check {
		return "", ErrorPasswordNotMatch
	}

	return pswd, nil
}

// COMMANDS

func adminOperation(t *TUI, cmd Command) {
	data, _ := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	arr := make([][]byte, 0, len(args)-1)

	var op spec.Admin
	switch args[0] {
	case "shutdown":
		op = spec.AdminShutdown
		offset, err := strconv.Atoi(args[1])
		if err != nil {
			cmd.print(err.Error(), cmds.ERROR)
			return
		}

		shutdown := time.Now().Add(
			time.Duration(offset) * time.Minute,
		)
		unix := spec.UnixStampToBytes(shutdown)
		arr = append(arr, unix)
	case "ban":
		op = spec.AdminDeregister
		arr = append(arr, []byte(args[1]))
	case "kick":
		op = spec.AdminDisconnect
		arr = append(arr, []byte(args[1]))
	case "setperms":
		op = spec.AdminChangePerms
		num, err := strconv.Atoi(args[2])
		if err != nil {
			cmd.print(err.Error(), cmds.ERROR)
			return
		}

		perms := spec.PermissionToBytes(uint(num))
		arr = append(arr, []byte(args[1]))
		arr = append(arr, perms)
	case "motd":
		op = spec.AdminMotd
		arr = append(arr, []byte(args[1]))
	default:
		cmd.print(ErrorInvalidArgument.Error(), cmds.ERROR)
		return
	}

	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	_, err := cmds.Admin(ctx, c, op, arr...)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
	}
}

func deregisterUser(t *TUI, cmd Command) {
	data, ok := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
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
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	_, err = cmds.Dereg(ctx, c, args[0], pswd)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}
}

func unsubEvent(t *TUI, cmd Command) {
	data, ok := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	if !ok {
		cmd.print(ErrorOffline.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	_, err := cmds.Unsub(ctx, c, args[0])

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}
}

func subEvent(t *TUI, cmd Command) {
	data, ok := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	if !ok {
		cmd.print(ErrorOffline.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	_, err := cmds.Sub(ctx, c, args[0])

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}
}

func exportKey(t *TUI, cmd Command) {
	data, _ := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	pswd, err := newLoginPopup(t, "Enter the account's password...")
	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	_, err = cmds.Export(c, args[0], pswd)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}
}

func importKey(t *TUI, cmd Command) {
	data, _ := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	pswd, err := askForNewPassword(t)
	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	_, err = cmds.Import(c, args[0], pswd, args[1])

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}
}

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

	var useTLS bool
	switch args[0] {
	case "on":
		useTLS = true
	case "off":
		useTLS = false
	default:
		cmd.print(ErrorInvalidArgument.Error(), cmds.ERROR)
		return
	}

	_, err := cmds.TLS(c, c.Data.Server, useTLS)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
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

	c, _ := cmd.createCmd(t, data)
	_, err := cmds.Discn(c)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
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

	c, _ := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	_, err := cmds.Logout(ctx, c)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

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
	lCtx, lCancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(lCancel)
	_, err = cmds.Login(lCtx, c, args[0], pswd)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	uname := data.User.User.Username
	t.comp.input.SetLabel(unameLabel(uname))
	if !t.status.showingUsers {
		toggleUserlist(t)
	}

	ctx, cancel := context.WithCancel(cmd.serv.Context().Get())
	data.Logout = cancel
	go t.receiveMessages(ctx, cmd.serv)
	go t.receiveHooks(ctx, cmd.serv)

	cmd.print("recovering messages...", cmds.INTERMEDIATE)
	rCtx, rCancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(rCancel)
	_, err = cmds.Reciv(rCtx, c)
	if err != nil {
		if errors.Is(err, spec.ErrorEmpty) {
			cmd.print("no new messages have been received", cmds.RESULT)
		} else {
			cmd.print(err.Error(), cmds.ERROR)
			return
		}
	}

	cmd.print("subscribing to relevant events...", cmds.INTERMEDIATE)

	output := cmd.print
	if !t.data.Verbose {
		output = func(string, cmds.OutputType) {}
	}

	defaultSubscribe(t, cmd.serv, output)
}

func listUsers(t *TUI, cmd Command) {
	data, _ := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)

	queryPerms := false
	if len(args) > 2 &&
		args[0] == "remote" &&
		args[2] == "-perms" {
		queryPerms = true
	}

	var usrs cmds.USRSType
	switch args[0] + "|" + args[1] {
	case "remote|all":
		if queryPerms {
			usrs = cmds.ALLPERMS
		} else {
			usrs = cmds.ALL
		}
	case "remote|online":
		if queryPerms {
			usrs = cmds.ONLINEPERMS
		} else {
			usrs = cmds.ONLINE
		}
	case "local|all":
		usrs = cmds.LOCAL_ALL
	case "local|server":
		usrs = cmds.LOCAL_SERVER
	default:
		cmd.print(ErrorInvalidArgument.Error(), cmds.ERROR)
		return
	}

	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	reply, err := cmds.Usrs(ctx, c, usrs)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	var list strings.Builder
	mode := fmt.Sprintf("%s %s", args[0], args[1])
	if queryPerms {
		list.WriteString("Showing " + mode + " users with permissions:\n")
	} else {
		list.WriteString("Showing " + mode + " users:\n")
	}

	if len(reply) == 0 {
		list.WriteString("No users to be shown.\n")
	}

	for _, v := range reply {
		uname, extra, ok := strings.Cut(string(v), " ")
		var str string
		if !ok {
			str = fmt.Sprintf(
				"- [pink::i]%s[-::-]\n",
				uname,
			)
		} else {
			str = fmt.Sprintf(
				"- [pink::i]%s[-::-] | [blue::b]%s[-::-]\n",
				uname, extra,
			)
		}
		list.WriteString(str)
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

	pswd, err := askForNewPassword(t)
	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	_, err = cmds.Reg(ctx, c, args[0], pswd)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}
}

func connectServer(t *TUI, cmd Command) {
	addr := cmd.serv.Source()
	if addr == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	data, ok := cmd.serv.Online()
	if ok {
		cmd.print(ErrorAlreadyOnline.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)

	var noVerify bool
	if len(args) >= 1 && args[0] == "-noverify" {
		noVerify = true
	} else {
		noVerify = false
	}

	cmd.print("attempting to connect...", cmds.INTERMEDIATE)
	_, err := cmds.Conn(c, *c.Data.Server, noVerify)

	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	cmd.serv.Context().Set(context.Background())
	t.comp.servers.SetSelectedTextColor(tcell.ColorGreen)

	c.Output = t.systemMessage("", defaultBuffer)
	go cmds.ListenPackets(c, func() {
		cmd.serv.Buffers().Offline()
		c.Data.Waitlist.Cancel(data.Logout)
		c.Data.Waitlist.Cancel(cmd.serv.Context().Cancel)

		t.comp.input.SetLabel(defaultLabel)
		t.comp.servers.SetSelectedTextColor(tcell.ColorPurple)

		cleanupSession(t, cmd.serv)
		cmd.serv.Notifications().Clear()

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
