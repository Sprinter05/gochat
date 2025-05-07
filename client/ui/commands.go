package ui

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Command struct {
	Operation string
	Arguments []string
	Server    net.Addr
}

type operation func(*TUI, Command)

var commands map[string]operation = map[string]operation{
	"list": listBuffers,
}

func (t *TUI) parseCommand(text string) {
	parts := strings.Split(text, " ")

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
