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

type operation func(*TUI, Command)

var commands map[string]operation = map[string]operation{
	"list":    listBuffers,
	"connect": connectServer,
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
	}

	fun, ok := commands[cmd.Operation]
	if !ok {
		t.showError(ErrorInvalidCmd)
		return
	}

	go fun(t, cmd)
}

// COMMANDS

// TODO: handle disconnection
func connectServer(t *TUI, cmd Command) {
	print := t.systemMessage()
	addr := t.Active().Source()
	if addr == nil {
		print(ErrorLocal.Error())
		return
	}

	parts := strings.Split(addr.String(), ":")
	data, ok := t.Active().Online()
	if ok {
		print(ErrorAlreadyOnline.Error())
		return
	}

	print("attempting to connect...")
	r := cmds.Conn(&cmds.CmdArgs{
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
