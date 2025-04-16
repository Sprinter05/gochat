package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("error: not enough arguments")
		return
	}

	// Gets .env pathname
	err := godotenv.Load(os.Args[1])
	if err != nil {
		log.Fatal("error: invalid .env path")
		return
	}

}
