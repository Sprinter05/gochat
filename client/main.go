package main

import (
	"encoding/json"
	"log"
	"net"
	"os"

	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
)

// Stores the json attributes of the client configuration file
type Config struct {
	Server struct {
		Address string `json:"address"`
		Port    uint16 `json:"port"`
	} `json:"server"`
	Database struct {
		Path     string `json:"path"`
		LogPath  string `json:"log_path"`
		LogLevel uint8  `json:"log_level"`
	} `json:"database"`
}

// Main client function
func main() {
	// Reads configuration file
	config := getConfig()

	address := config.Server.Address
	port := config.Server.Port

	// Opens the database
	dbLog := db.GetDBLogger(config.Database.LogLevel, config.Database.LogPath)
	clientDB := db.OpenClientDatabase(config.Database.Path, dbLog)

	var cl spec.Connection
	var con net.Conn
	if address != "" {
		var conErr error
		con, conErr = commands.Connect(address, port)
		if conErr != nil {
			log.Fatal(conErr)
		}
	}
	cl = spec.Connection{Conn: con}

	server := db.SaveServer(clientDB, address, port)
	// TODO: verbose to config
	static := commands.StaticData{Verbose: true, Output: ShellPrint, DB: clientDB}
	data := commands.Data{ClientCon: cl, Server: server, Static: &static}

	if address != "" {
		commands.ConnectionStart(&data)
	}

	// go Listen(&data)
	NewShell(&data)
	if data.ClientCon.Conn != nil {
		data.ClientCon.Conn.Close()
	}
}

// Returns a Config struct with the data obtained from the json
// configuration file.
func getConfig() Config {
	config := Config{}

	f, err := os.Open("client_config.json")
	if err != nil {
		log.Fatalf("configuration file could not be opened: %s", err)
	}
	defer f.Close()

	jsonParser := json.NewDecoder(f)
	jsonParser.Decode(&config)
	return config
}
