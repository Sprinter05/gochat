package main

import (
	"log"

	"github.com/Sprinter05/gochat/client/ui"
)

func main() {
	_, app := ui.New()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
