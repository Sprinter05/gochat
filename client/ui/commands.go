package ui

import (
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
		format: "/connect",
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
}

func (t *TUI) parseCommand(text string) {
	lower := strings.ToLower(text)
	parts := strings.Split(lower, " ")

	if parts[0] == "" {
		t.showError(ErrorEmptyCmd)
		return
	}

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
	r := cmds.Discn(c, args...)

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
	r := cmds.Logout(c, args...)

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}

	t.comp.input.SetLabel(defaultLabel)
}

func loginUser(t *TUI, cmd Command) {
	data, ok := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	if data.IsUserLoggedIn() {
		cmd.print(ErrorLoggedIn.Error(), cmds.ERROR)
		return
	}

	if !ok {
		cmd.print(ErrorOffline.Error(), cmds.ERROR)
		return
	}

	pswd, err := newLoginPopup(t)
	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	r := cmds.Login(c, args[0], []byte(pswd))

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}

	uname := data.User.User.Username
	t.comp.input.SetLabel(unameLabel(uname))
}

func listUsers(t *TUI, cmd Command) {
	data, _ := cmd.serv.Online()
	if data == nil {
		cmd.print(ErrorLocalServer.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	r := cmds.Usrs(c, args...)

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

	pswd, err := newLoginPopup(t)
	if err != nil {
		cmd.print(err.Error(), cmds.ERROR)
		return
	}

	c, args := cmd.createCmd(t, data)
	r := cmds.Reg(c, args[0], []byte(pswd))

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}
}

// TODO: handle disconnection
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

	cmd.print("attempting to connect...", cmds.INTERMEDIATE)
	c, _ := cmd.createCmd(t, data)
	r := cmds.Conn(c, []byte(parts[0]), []byte(parts[1]))

	if r.Error != nil {
		cmd.print(r.Error.Error(), cmds.ERROR)
		return
	}

	t.comp.servers.SetSelectedTextColor(tcell.ColorGreen)
}

func listBuffers(t *TUI, cmd Command) {
	var list strings.Builder
	bufs := cmd.serv.Buffers()
	l := bufs.tabs.GetAll()

	list.WriteString("Showing active server buffers: ")
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
