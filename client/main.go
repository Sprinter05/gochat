package main

import (
	"log"

	"github.com/Sprinter05/gochat/client/ui"
)

func main() {
	_, app := ui.NewTUI()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
