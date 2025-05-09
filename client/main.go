package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"os"

	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/client/ui"
	"github.com/Sprinter05/gochat/internal/spec"
	"gorm.io/gorm"
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

var (
	configFile   string
	useShell     bool
	verbosePrint bool
)

func init() {
	//! Temporary
	log.SetOutput(io.Discard)

	flag.StringVar(&configFile, "config", "config.json", "Configuration file to use. Must be in JSON format.")
	flag.BoolVar(&useShell, "shell", false, "Whether to use a shell instead of a TUI.")
	flag.BoolVar(&verbosePrint, "verbose", true, "Whether or not to print verbose output information.")
	flag.Parse()
}

// Main client function
func main() {
	// Reads configuration file
	config := getConfig()

	// Opens the database
	dbLog := db.GetDBLogger(config.Database.LogLevel, config.Database.LogPath)
	clientDB := db.OpenClientDatabase(config.Database.Path, dbLog)

	if useShell {
		setupShell(config, clientDB)
	} else {
		setupTUI(clientDB)
	}
}

// Returns a Config struct with the data obtained from the json
// configuration file.
func getConfig() Config {
	config := Config{}

	f, err := os.Open(configFile)
	if err != nil {
		log.Fatalf("configuration file could not be opened: %s", err)
	}
	defer f.Close()

	jsonParser := json.NewDecoder(f)
	jsonParser.Decode(&config)
	return config
}

func setupTUI(dbconn *gorm.DB) {
	_, app := ui.New(commands.StaticData{
		Verbose: verbosePrint,
		DB:      dbconn,
	})

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func setupShell(config Config, dbconn *gorm.DB) {
	address := config.Server.Address
	port := config.Server.Port

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

	server := db.SaveServer(dbconn, address, port)
	// TODO: verbose to config
	static := commands.StaticData{Verbose: verbosePrint, DB: dbconn}
	data := commands.Data{ClientCon: cl, Server: server}
	args := commands.CmdArgs{Data: &data, Static: &static, Output: ShellPrint}

	if address != "" {
		commands.ConnectionStart(&args)
	}

	// go Listen(&data)
	NewShell(&args)
	if data.ClientCon.Conn != nil {
		data.ClientCon.Conn.Close()
	}
}
