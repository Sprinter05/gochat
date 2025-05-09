package ui

import (
	"fmt"
	"strings"
	"time"

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

func connectServer(t *TUI, cmd Command) {
	s := t.Active()
	curr := t.Buffer()

	addr := s.Source()
	if addr == nil {
		t.showError(ErrorLocal)
		return
	}

	parts := strings.Split(addr.String(), ":")
	data, ok := s.Online()
	if ok {
		t.showError(ErrorAlreadyOnline)
		return
	}

	r := cmds.Conn(&cmds.CmdArgs{
		Data:   data,
		Static: t.data,
		Output: func(text string) {
			t.SendMessage(Message{
				Buffer:    curr,
				Sender:    "System",
				Content:   text,
				Timestamp: time.Now(),
				Source:    addr,
			})
		},
	}, []byte(parts[0]), []byte(parts[1]))

	if r.Error != nil {
		t.showError(r.Error)
		return
	}

	t.comp.servers.SetSelectedTextColor(tcell.ColorGreen)
}

func listBuffers(t *TUI, cmd Command) {
	var list strings.Builder
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

	t.SendMessage(Message{
		Buffer:    t.Buffer(),
		Sender:    "System",
		Content:   content[:len(content)-1],
		Timestamp: time.Now(),
		Source:    t.Active().Source(),
	})
}
