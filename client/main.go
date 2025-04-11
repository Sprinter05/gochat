package main

import (
	"log"

	"github.com/Sprinter05/gochat/client/ui"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()
	tui := ui.NewTUI()
	init := app.SetRoot(tui.Area, true).SetFocus(tui.Area)

	if err := init.Run(); err != nil {
		log.Fatal(err)
	}
}
