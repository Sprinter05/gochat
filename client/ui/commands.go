package ui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/gdamore/tcell/v2"
)

/* TYPES */

// Defines a command to be ran in the TUI
type Command struct {
	Operation string   // Name of the command
	Arguments []string // List of arguments

	serv  Server          // Target server of the commnd
	print cmds.OutputFunc // Printing function
}

// Defines the function of the command to run
type operationFunc func(*TUI, Command) error

// Struct to define operations in the shell
type operation struct {
	fun    operationFunc // Function to be ran
	nArgs  uint          // Number of arguments needed
	format string        // Format of the command
}

// List of commands that can be ran
var commands map[string]operation = map[string]operation{
	"version": {
		fun:    showVersion,
		nArgs:  0,
		format: "/version",
	},
	"servers": {
		fun:    listServers,
		nArgs:  0,
		format: "/servers",
	},
	"buffers": {
		fun:    listBuffers,
		nArgs:  0,
		format: "/buffers",
	},
	"clear": {
		fun:    clearSystem,
		nArgs:  0,
		format: "/clear",
	},
	"config": {
		fun:    showConfig,
		nArgs:  0,
		format: "/config",
	},
	"set": {
		fun:    setConfig,
		nArgs:  2,
		format: "/set <option> <value>",
	},
	"connect": {
		fun:    connectServer,
		nArgs:  0,
		format: "/connect (-noverify) (-noidle)",
	},
	"register": {
		fun:    registerUser,
		nArgs:  1,
		format: "/register <username>",
	},
	"deregister": {
		fun:    deregisterUser,
		nArgs:  1,
		format: "/deregister <user>",
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
	"users": {
		fun:    listUsers,
		nArgs:  2,
		format: "/users <remote/local> <all/online/server> (-perms)",
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
	"admin": {
		fun:    adminOperation,
		nArgs:  1,
		format: "/admin <operation> <arg_1> <arg_2> ... <arg_n>",
	},
	"recover": {
		fun:    recoverData,
		nArgs:  1,
		format: "/recover <username> (-cleanup)",
	},
}

// Parses a shell command to be ran
func (t *TUI) parseCommand(text string) {
	parts := strings.Split(text, " ")

	if parts[0] == "" {
		t.showError(ErrorEmptyCmd)
		return
	}

	t.history.Add(text)

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

	// If we didnt give enough arguments we print the format
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

	// Run concurrently
	go func() {
		err := op.fun(t, cmd)
		if err != nil {
			cmd.print(err.Error(), cmds.ERROR)
		}
	}()
}

/* AUXILIARY */

// Creates the structs necessary to use in commands
func (c Command) createCmd(t *TUI, d *cmds.Data) (cmds.Command, []string) {
	return cmds.Command{
		Data:   d,
		Static: t.static(),
		Output: c.print,
	}, c.Arguments
}

// Asks for a new password by asking for the password
// and repeating it
func askForNewPassword(t *TUI) (string, error) {
	pswd, err := newPasswordPopup(t, "Enter a password...")
	if err != nil {
		return "", err
	}

	check, err := newPasswordPopup(t, "Repeat your password...")
	if err != nil {
		return "", err
	}

	if pswd != check {
		return "", ErrorPasswordNotMatch
	}

	return pswd, nil
}

// Returns the list of structs to be shown in the configuration
func configList(t *TUI, s Server) []cmds.ConfigObj {
	data, _ := s.Online()
	list := make([]cmds.ConfigObj, 0)

	list = append(list, cmds.ConfigObj{
		Prefix: "TUI",
		Object: &t.params,
		Finish: func() {
			renderBuflist(t)
			renderUserlist(t)
		},
	})

	if data != nil {
		list = append(list, cmds.ConfigObj{
			Prefix: "Server",
			Object: data.Server,
			Precondition: func() error {
				if data.IsConnected() {
					return ErrorOffline
				}
				return nil
			},
			Update: db.UpdateServer,
			Finish: func() {
				updateServers(t)
			},
		})
	}

	return list
}

/* COMMANDS */

func showVersion(t *TUI, cmd Command) error {
	str := fmt.Sprintf(
		"\n* Client TUI version: [orange::i]v%.1f[-::-]\n* Protocol version: [orange::i]v%d[-::-]",
		tuiVersion,
		spec.ProtocolVersion,
	)
	cmd.print(str, cmds.RESULT)
	return nil
}

func listServers(t *TUI, cmd Command) error {
	var list strings.Builder
	servs, err := db.GetAllServers(t.db)
	if err != nil {
		return err
	}

	for _, v := range servs {
		hidden := ""
		_, ok := t.servers.Get(v.Name)
		if !ok {
			hidden = " - [gray::i]Hidden[-::-]"
		}

		addr := Source{
			Address: v.Address,
			Port:    v.Port,
		}

		str := fmt.Sprintf(
			"\n- [yellow::b]%s[-::-] ([red]%s[-])%s",
			v.Name, addr.String(), hidden,
		)

		list.WriteString(str)
	}

	content := list.String()
	cmd.print(content, cmds.RESULT)
	return nil
}

func listBuffers(t *TUI, cmd Command) error {
	var list strings.Builder
	bufs := cmd.serv.Buffers()
	l := bufs.tabs.GetAll()

	if len(l) == 0 {
		cmd.print("no buffers to show", cmds.RESULT)
		return nil
	}

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
	return nil
}

func clearSystem(t *TUI, cmd Command) error {
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

	return nil
}

func showConfig(t *TUI, cmd Command) error {
	objs := configList(t, cmd.serv)
	list := cmds.CONFIG(objs...)

	if len(list) == 0 {
		cmd.print("No configuration options to show", cmds.RESULT)
		return nil
	}

	var str strings.Builder
	str.WriteString("Showing configuration options:")
	for _, v := range list {
		name, val, _ := strings.Cut(string(v), " = ")

		format := fmt.Sprintf(
			"\n- [pink::i]%s[-::-] = [blue::b]%s[-::-]",
			name, val,
		)
		str.WriteString(format)
	}

	cmd.print(str.String(), cmds.RESULT)
	return nil
}

func setConfig(t *TUI, cmd Command) error {
	data, _ := cmd.serv.Online()
	c, args := cmd.createCmd(t, data)

	extra := args[1:]
	extended := strings.Join(extra, " ")

	objs := configList(t, cmd.serv)
	err := cmds.SET(c, args[0], extended, objs...)
	if err != nil {
		return err
	}

	return nil
}

func connectServer(t *TUI, cmd Command) error {
	addr := cmd.serv.Source()
	if addr == nil {
		return ErrorLocalServer
	}

	data, ok := cmd.serv.Online()
	if ok {
		return ErrorAlreadyOnline
	}

	c, args := cmd.createCmd(t, data)

	var noVerify bool
	if slices.Contains(args, "-noverify") {
		noVerify = true
	} else {
		noVerify = false
	}

	cmd.print("attempting to connect...", cmds.INTERMEDIATE)
	err := cmds.CONN(c, *c.Data.Server, noVerify)
	if err != nil {
		return err
	}

	cmd.serv.Context().Create(context.Background())
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

	// Prevent idle
	if slices.Contains(args, "-noidle") {
		cmd.print("running hook to prevent idle disconnection", cmds.SECONDARY)

		go cmds.PreventIdle(
			cmd.serv.Context().Get(),
			c.Data,
			time.Duration(spec.ReadTimeout-1)*time.Minute,
		)
	}

	return nil
}

func registerUser(t *TUI, cmd Command) error {
	data, ok := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	if !ok {
		return ErrorOffline
	}

	pswd, err := askForNewPassword(t)
	if err != nil {
		return err
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	err = cmds.REG(ctx, c, args[0], pswd)
	if err != nil {
		return err
	}

	return nil
}

func deregisterUser(t *TUI, cmd Command) error {
	data, ok := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	if !ok {
		return ErrorOffline
	}

	pswd, err := newPasswordPopup(t, "Enter the account's password...")
	if err != nil {
		return err
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	err = cmds.DEREG(ctx, c, args[0], pswd)
	if err != nil {
		return err
	}

	return nil
}

func importKey(t *TUI, cmd Command) error {
	data, _ := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	pswd, err := askForNewPassword(t)
	if err != nil {
		return err
	}

	c, args := cmd.createCmd(t, data)
	err = cmds.IMPORT(c, args[0], pswd, args[1])
	if err != nil {
		return err
	}

	return nil
}

func exportKey(t *TUI, cmd Command) error {
	data, _ := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	pswd, err := newPasswordPopup(t, "Enter the account's password...")
	if err != nil {
		return err
	}

	c, args := cmd.createCmd(t, data)
	err = cmds.EXPORT(c, args[0], pswd)
	if err != nil {
		return err
	}

	return nil
}

func loginUser(t *TUI, cmd Command) error {
	data, ok := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	if data.IsLoggedIn() {
		return ErrorLoggedIn
	}

	if !ok {
		return ErrorOffline
	}

	pswd, err := newPasswordPopup(t, "Enter the account's password...")
	if err != nil {
		return err
	}

	c, args := cmd.createCmd(t, data)
	lCtx, lCancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(lCancel)
	err = cmds.LOGIN(lCtx, c, args[0], pswd)
	if err != nil {
		return err
	}

	uname := data.LocalUser.User.Username
	t.comp.input.SetLabel(unameLabel(uname))
	if !t.status.showingUsers {
		toggleUserlist(t)
	}

	ctx, cancel := context.WithCancel(cmd.serv.Context().Get())
	data.Logout = cancel

	go t.receiveMessages(ctx, cmd.serv)
	go t.receiveHooks(ctx, cmd.serv)
	go t.waitShutdown(ctx, cmd.serv)

	cmd.print("recovering messages...", cmds.INTERMEDIATE)
	rCtx, rCancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(rCancel)
	err = cmds.RECIV(rCtx, c)
	if err != nil {
		if errors.Is(err, spec.ErrorEmpty) {
			cmd.print("no new messages have been received", cmds.RESULT)
		} else {
			return err
		}
	}

	cmd.print("subscribing to relevant events...", cmds.INTERMEDIATE)

	output := cmd.print
	if !t.params.Verbose {
		output = func(string, cmds.OutputType) {}
	}

	defaultSubscribe(t, cmd.serv, output)

	return nil
}

func logoutUser(t *TUI, cmd Command) error {
	data, _ := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	c, _ := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	err := cmds.LOGOUT(ctx, c)
	if err != nil {
		return err
	}

	t.comp.input.SetLabel(defaultLabel)
	cleanupSession(t, cmd.serv)

	return nil
}

func disconnectServer(t *TUI, cmd Command) error {
	data, _ := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	c, _ := cmd.createCmd(t, data)
	err := cmds.DISCN(c)
	if err != nil {
		return err
	}

	t.comp.input.SetLabel(defaultLabel)
	t.comp.servers.SetSelectedTextColor(tcell.ColorPurple)

	return nil
}

func listUsers(t *TUI, cmd Command) error {
	data, _ := cmd.serv.Online()
	opt := cmd.Arguments[0] + "|" + cmd.Arguments[1]
	if data == nil && opt != "local|all" {
		return ErrorLocalServer
	}

	c, args := cmd.createCmd(t, data)

	queryPerms := false
	if len(args) > 2 &&
		args[0] == "remote" &&
		slices.Contains(args[2:], "-perms") {
		queryPerms = true
	}

	var usrs cmds.USRSType
	switch opt {
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
		return ErrorInvalidArgument
	}

	ctx := context.Background()
	if opt != "local|all" {
		var cancel context.CancelFunc
		ctx, cancel = timeout(cmd.serv, c.Data)
		defer c.Data.Waitlist.Cancel(cancel)
	}
	reply, err := cmds.USRS(ctx, c, usrs)
	if err != nil {
		return err
	}

	var list strings.Builder
	mode := fmt.Sprintf("%s %s", args[1], args[0])
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

	return nil
}

func subEvent(t *TUI, cmd Command) error {
	data, ok := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	if !ok {
		return ErrorOffline
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	err := cmds.SUB(ctx, c, args[0])
	if err != nil {
		return err
	}

	return nil
}

func unsubEvent(t *TUI, cmd Command) error {
	data, ok := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	if !ok {
		return ErrorOffline
	}

	c, args := cmd.createCmd(t, data)
	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)
	err := cmds.UNSUB(ctx, c, args[0])
	if err != nil {
		return err
	}

	return nil
}

func adminOperation(t *TUI, cmd Command) error {
	data, _ := cmd.serv.Online()
	if data == nil {
		return ErrorLocalServer
	}

	c, args := cmd.createCmd(t, data)

	ctx, cancel := timeout(cmd.serv, c.Data)
	defer c.Data.Waitlist.Cancel(cancel)

	extra := make([][]byte, 0, len(args)-1)
	list := args[1:]
	for _, v := range list {
		extra = append(extra, []byte(v))
	}

	err := cmds.ADMIN(ctx, c, args[0], extra...)
	if err != nil {
		return err
	}

	return nil
}

func recoverData(t *TUI, cmd Command) error {
	uname := cmd.Arguments[0]
	pswd, err := newPasswordPopup(t, "Please enter the account's password...")
	if err != nil {
		return err
	}

	cleanup := false
	if slices.Contains(cmd.Arguments[1:], "-cleanup") {
		cleanup = true
	}

	err = cmds.RECOVER(cmds.Command{
		Static: t.static(),
		Output: cmd.print,
	}, uname, pswd, cleanup)
	if err != nil {
		return err
	}

	cmd.print("data succesfully recovered!", cmds.RESULT)

	return nil
}
