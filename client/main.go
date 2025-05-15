package main

import (
	"encoding/json"
	"flag"
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
	ShellServer struct {
		Address string `json:"address"`
		Port    uint16 `json:"port"`
	} `json:"shell_server"`
	Database struct {
		Path     string `json:"path"`
		LogPath  string `json:"log_path"`
		LogLevel uint8  `json:"log_level"`
	} `json:"database"`
	UIConfig struct {
		DebugBuffer bool `json:"debug_buffer"`
	} `json:"ui_config"`
}

var (
	configFile   string
	useShell     bool
	verbosePrint bool
)

func init() {
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
		setupTUI(config, clientDB)
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

func setupTUI(config Config, dbconn *gorm.DB) {
	_, app := ui.New(commands.StaticData{
		Verbose: verbosePrint,
		DB:      dbconn,
	}, config.UIConfig.DebugBuffer)

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func setupShell(config Config, dbconn *gorm.DB) {
	address := config.ShellServer.Address
	port := config.ShellServer.Port

	var cl spec.Connection
	var con net.Conn
	var server db.Server
	if address != "" {
		var conErr error
		con, conErr = commands.Connect(address, port)
		if conErr != nil {
			log.Fatal(conErr)
		}
		server, _ = db.SaveServer(dbconn, address, port, "Default")
	}
	cl = spec.Connection{Conn: con}

	// TODO: verbose to config
	static := commands.StaticData{Verbose: verbosePrint, DB: dbconn}
	data := commands.Data{ClientCon: cl, Server: server}
	args := commands.Command{Data: &data, Static: static, Output: ShellPrint}

	if address != "" {
		commands.ConnectionStart(args)
	}

	// go Listen(&data)
	NewShell(args)
	if data.ClientCon.Conn != nil {
		data.ClientCon.Conn.Close()
	}
}
