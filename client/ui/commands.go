package ui

import (
	"fmt"
	"strings"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/gdamore/tcell/v2"
)

type Command struct {
	Operation string
	Arguments []string

	serv  Server
	print func(string)
}

type operation struct {
	fun    func(*TUI, Command)
	nArgs  uint
	format string
}

var commands map[string]operation = map[string]operation{
	"list": {
		fun:    listBuffers,
		nArgs:  0,
		format: "/list",
	},
	"connect": {
		fun:    connectServer,
		nArgs:  0,
		format: "/connect",
	},
	"register": {
		fun:    registerUser,
		nArgs:  2,
		format: "/register <username> <password>",
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
}

func (t *TUI) parseCommand(text string) {
	parts := strings.Split(text, " ")

	operation := strings.ToLower(parts[0])
	if operation == "" {
		t.showError(ErrorEmptyCmd)
		return
	}

	cmd := Command{
		Operation: operation,
		Arguments: parts[1:],
		serv:      t.Active(),
		print:     t.systemMessage(),
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
		cmd.print(str)
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
		Static: t.data,
		Output: c.print,
	}, array
}

// COMMANDS

func loginUser(t *TUI, cmd Command) {
	data, ok := cmd.serv.Online()
	if !ok {
		cmd.print(ErrorOffline.Error())
		return
	}

	pswd, err := newLoginPopup(t)
	if err != nil {
		cmd.print(err.Error())
		return
	}

	c, args := cmd.createCmd(t, data)
	r := cmds.Login(c, args[0], []byte(pswd))

	if r.Error != nil {
		cmd.print(r.Error.Error())
		return
	}
}

func listUsers(t *TUI, cmd Command) {
	data, ok := cmd.serv.Online()
	if !ok && cmd.Arguments[0] != "local" {
		cmd.print(ErrorOffline.Error())
		return
	}

	if data == nil {
		cmd.print(ErrorLocalServer.Error())
		return
	}

	c, args := cmd.createCmd(t, data)
	r := cmds.Usrs(c, args...)

	if r.Error != nil {
		cmd.print(r.Error.Error())
		return
	}

	var list strings.Builder
	list.WriteString("Showing " + cmd.Arguments[0] + " users:\n")
	for _, v := range r.Arguments {
		list.WriteString("- [pink::i]" + string(v) + "[-::-]\n")
	}

	l := list.Len()
	cmd.print(list.String()[:l-1])
}

func registerUser(t *TUI, cmd Command) {
	data, ok := cmd.serv.Online()
	if !ok {
		cmd.print(ErrorOffline.Error())
		return
	}

	c, args := cmd.createCmd(t, data)
	r := cmds.Reg(c, args...)

	if r.Error != nil {
		cmd.print(r.Error.Error())
		return
	}
}

// TODO: handle disconnection
func connectServer(t *TUI, cmd Command) {
	addr := cmd.serv.Source()
	if addr == nil {
		cmd.print(ErrorLocalServer.Error())
		return
	}

	parts := strings.Split(addr.String(), ":")
	data, ok := cmd.serv.Online()
	if ok {
		cmd.print(ErrorAlreadyOnline.Error())
		return
	}

	cmd.print("attempting to connect...")
	c, _ := cmd.createCmd(t, data)
	r := cmds.Conn(c, []byte(parts[0]), []byte(parts[1]))

	if r.Error != nil {
		cmd.print(r.Error.Error())
		return
	}

	t.comp.servers.SetSelectedTextColor(tcell.ColorGreen)
}

func listBuffers(t *TUI, cmd Command) {
	var list strings.Builder
	bufs := cmd.serv.Buffers()
	l := bufs.tabs.GetAll()

	for i, v := range l {
		hidden := ""
		if v.index == -1 {
			hidden = " - [gray::i]Hidden[-::-]"
		}

		str := fmt.Sprintf(
			"[green]%d:[-::-] %s%s\n",
			i+1, v.name, hidden,
		)

		list.WriteString(str)
	}

	content := list.String()

	cmd.print(content[:len(content)-1])
}
