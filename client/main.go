package main

// Main gochat client package

import (
	"encoding/json"
	"errors"
	"flag"
	"io/fs"
	"log"
	"net"
	"os"

	"github.com/Sprinter05/gochat/client/cli"
	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/client/ui"
	"gorm.io/gorm"
)

/* CONFIG */

// Specifies the configuration JSON file for
// the client.
type Config struct {
	ShellServer struct {
		Address    string `json:"address"`
		Port       uint16 `json:"port"`
		TLS        bool   `json:"use_tls"`
		VerifyCert bool   `json:"verify_tls"`
	} `json:"shell_server"`
	Database struct {
		Path     string `json:"path"`
		LogPath  string `json:"log_path"`
		LogLevel uint8  `json:"log_level"` // From 1 to 4
	} `json:"database"`
	UIConfig struct {
		DebugBuffer bool `json:"debug_buffer"`
	} `json:"ui_config"`
}

// Returns a Config struct with the data obtained from the json
// configuration file.
func getConfig() (config Config) {
	f, err := os.Open(configFile)
	if err != nil {
		// Get the default configuration
		config = defaultConfig()
		cfg, err := json.Marshal(config)
		if err != nil {
			log.Fatal(err)
		}

		// Write it for next execution
		err = os.WriteFile("config.json", cfg, commands.DefaultPerms)
		if err != nil {
			log.Fatal(err)
		}

		return config
	}
	defer f.Close()

	// Decode the configuration into the struct
	jsonParser := json.NewDecoder(f)
	jsonParser.Decode(&config)
	return config
}

// Returns the default configuration file
func defaultConfig() Config {
	return Config{
		Database: struct {
			Path     string "json:\"path\""
			LogPath  string "json:\"log_path\""
			LogLevel uint8  "json:\"log_level\""
		}{
			Path:     "client.db",
			LogPath:  "logs/database.log",
			LogLevel: 2,
		},
	}
}

/* FLAGS */

// Specifies flags to be passed to the program
var (
	configFile   string
	useShell     bool
	verbosePrint bool
)

// Function that is ran every time the program is started
func init() {
	flag.StringVar(&configFile, "config", "config.json", "Configuration file to use. Must be in JSON format.")
	flag.BoolVar(&useShell, "shell", false, "Whether to use a shell instead of a TUI.")
	flag.BoolVar(&verbosePrint, "verbose", false, "Whether or not to print verbose output information.")
	flag.Parse()

	folders := []string{
		"export",
		"import",
		"logs",
	}

	// Create necessary folders
	for _, v := range folders {
		_, err := os.Stat(v)
		if err != nil && errors.Is(err, fs.ErrNotExist) {
			err = os.Mkdir(v, commands.DefaultPerms)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

}

/* MAIN */

// Main client function
func main() {
	// Reads configuration file
	config := getConfig()

	// Opens the database
	dbLog := db.GetDBLogger(config.Database.LogLevel, config.Database.LogPath)
	clientDB := db.OpenDatabase(config.Database.Path, dbLog)

	if useShell {
		setupShell(config, clientDB)
	} else {
		setupTUI(config, clientDB)
	}
}

// Function that creates a new TUI and executes it
func setupTUI(config Config, dbconn *gorm.DB) {
	_, app := ui.New(commands.StaticData{
		Verbose: verbosePrint,
		DB:      dbconn,
	}, config.UIConfig.DebugBuffer && verbosePrint)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// Function that creates a new working shell and executes it
func setupShell(config Config, dbconn *gorm.DB) {
	address := config.ShellServer.Address
	port := config.ShellServer.Port

	var conn net.Conn
	var server db.Server

	// Connect automatically if shell server exists
	if address != "" {
		var conErr error
		conn, conErr = commands.SocketConnect(
			address, port,
			config.ShellServer.TLS,
			config.ShellServer.VerifyCert,
		)
		if conErr != nil {
			log.Fatal(conErr)
		}
		server, _ = db.AddServer(dbconn, address, port, "Default", false)
	}

	args := cli.New(commands.StaticData{
		Verbose: verbosePrint,
		DB:      dbconn,
	}, conn, server)

	cli.Run(args)
}
