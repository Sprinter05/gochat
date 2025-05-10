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
	lower := strings.ToLower(text)
	parts := strings.Split(lower, " ")

	if parts[0] == "" {
		t.showError(ErrorEmptyCmd)
		return
	}

	l := len(parts)
	cmd := Command{
		Operation: parts[0],
		Arguments: parts[1 : l-2],
		serv:      t.Active(),
		print:     t.systemMessage(),
	}

	op, ok := commands[cmd.Operation]
	if !ok {
		t.showError(ErrorInvalidCmd)
		return
	}

	if len(cmd.Arguments) < int(op.nArgs) {
		str := fmt.Sprintf("%s: %s", ErrorArguments, op.format)
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

func (c Command) printResult(arr ...[]byte) {
	for _, v := range arr {
		c.print(string(v))
	}
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

	c, args := cmd.createCmd(t, data)
	r := cmds.Usrs(c, args...)

	if r.Error != nil {
		cmd.print(r.Error.Error())
		return
	}

	cmd.printResult(r.Arguments...)
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
		cmd.print(ErrorLocal.Error())
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
