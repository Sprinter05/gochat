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
}

type operation struct {
	fun    func(*TUI, Command)
	nArgs  uint
	format string
}

var commands map[string]operation = map[string]operation{
	"list": operation{
		fun:    listBuffers,
		nArgs:  0,
		format: "/list",
	},
	"connect": operation{
		fun:    connectServer,
		nArgs:  0,
		format: "/connect",
	},
	"register": operation{
		fun:    registerUser,
		nArgs:  2,
		format: "/register <username> <password>",
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
	}

	op, ok := commands[cmd.Operation]
	if !ok {
		t.showError(ErrorInvalidCmd)
		return
	}

	if len(cmd.Arguments) < int(op.nArgs) {
		print := t.systemMessage()
		str := fmt.Sprintf("%s: %s", ErrorArguments, op.format)
		print(str)
		return
	}

	go op.fun(t, cmd)
}

// COMMANDS

func registerUser(t *TUI, cmd Command) {
	print := t.systemMessage()
	s := t.Active()
	data, ok := s.Online()

	if !ok {
		print(ErrorOffline.Error())
		return
	}

	r := cmds.Reg(cmds.Command{
		Data:   data,
		Static: t.data,
		Output: print,
	})

}

// TODO: handle disconnection
func connectServer(t *TUI, cmd Command) {
	print := t.systemMessage()
	s := t.Active()
	addr := s.Source()
	if addr == nil {
		print(ErrorLocal.Error())
		return
	}

	parts := strings.Split(addr.String(), ":")
	data, ok := s.Online()
	if ok {
		print(ErrorAlreadyOnline.Error())
		return
	}

	print("attempting to connect...")
	r := cmds.Conn(cmds.Command{
		Data:   data,
		Static: t.data,
		Output: print,
	}, []byte(parts[0]), []byte(parts[1]))

	if r.Error != nil {
		print(r.Error.Error())
		return
	}

	t.comp.servers.SetSelectedTextColor(tcell.ColorGreen)
}

func listBuffers(t *TUI, cmd Command) {
	var list strings.Builder
	print := t.systemMessage()
	bufs := t.Active().Buffers()
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

	print(content[:len(content)-1])
}
