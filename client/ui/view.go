package ui

import (
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/rivo/tview"
)

var (
	chat    *tview.Box        = tview.NewBox().SetBorder(true).SetTitleAlign(tview.AlignLeft)
	buffers *tview.Box        = tview.NewBox().SetBorder(true).SetTitleAlign(tview.AlignLeft)
	users   *tview.Box        = tview.NewBox().SetBorder(true).SetTitleAlign(tview.AlignLeft)
	input   *tview.InputField = tview.NewInputField().SetLabel("> ").SetAcceptanceFunc(tview.InputFieldInteger)
)

type TUI struct {
	Messages models.Table[string, models.Slice[string]]
	Area     *tview.Flex
	tabs     []string
	active   int
}

func NewTUI() *TUI {
	return &TUI{
		Messages: models.NewTable[string, models.Slice[string]](0),
		Area: tview.NewFlex().
			AddItem(buffers, 0, 1, false).
			AddItem(chat, 0, 4, false).SetDirection(tview.FlexRow).
			AddItem(input, 0, 1, true).SetDirection(tview.FlexColumn).
			AddItem(users, 0, 1, false),
		tabs:   make([]string, 0),
		active: -1,
	}
}
