package commands

import (
	"context"
	mrand "math/rand/v2"
	"net"
	"sync"

	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
	"gorm.io/gorm"
)

// Struct that contains all the data necessary to run a command
// Requires fields may change between commands
// Commands may alter the data if necessary
type Data struct {
	conn     net.Conn                      // Specifies the connection to the server
	server   *db.Server                    // Specifies the database server
	user     *db.LocalUser                 // Specifies the logged in user
	logout   context.CancelFunc            // Specifies the function to call on a logout for context propagation
	Waitlist models.Waitlist[spec.Command] // Stores all commands to be retrieved later
	token    string                        // Reusable token in case of TLS usage
	next     spec.ID                       // Specifies the next ID that should be used when sending a packet
	mut      sync.RWMutex
}

// Static data that should only be assigned
// in specific cases
type StaticData struct {
	Verbose bool     // Whether or not to print detailed information
	DB      *gorm.DB // Connection to the database
}

// Specifies all structs necessary for a command
type Command struct {
	Output OutputFunc  // Custom output-printing function
	Static *StaticData // Static Data (mostly)
	Data   *Data       // Modifiable Data
}

func (d *Data) GetConn() net.Conn {
	d.mut.Lock()
	defer d.mut.Unlock()
	return d.conn
}

func (d *Data) SetConn(c net.Conn) {
	d.mut.Lock()
	defer d.mut.Unlock()
	d.conn = c
}

func (d *Data) GetServer() *db.Server {
	d.mut.Lock()
	defer d.mut.Unlock()
	return d.server
}

func (d *Data) SetServer(s *db.Server) {
	d.mut.Lock()
	defer d.mut.Unlock()
	d.server = s
}

func (d *Data) GetUser() *db.LocalUser {
	d.mut.Lock()
	defer d.mut.Unlock()
	return d.user
}

func (d *Data) SetUser(u *db.LocalUser) {
	d.mut.Lock()
	defer d.mut.Unlock()
	d.user = u
}

func (d *Data) GetToken() string {
	d.mut.Lock()
	defer d.mut.Unlock()
	return d.token
}

func (d *Data) SetToken(t string) {
	d.mut.Lock()
	defer d.mut.Unlock()
	d.token = t
}

func (d *Data) GetLogout() context.CancelFunc {
	d.mut.Lock()
	defer d.mut.Unlock()
	return d.logout
}

func (d *Data) SetLogout(l context.CancelFunc) {
	d.mut.Lock()
	defer d.mut.Unlock()
	d.logout = l
}

// Creates a new empty but initialised struct for Data
func NewEmptyData() Data {
	initial := mrand.IntN(int(spec.MaxID))

	return Data{
		Waitlist: DefaultWaitlist(),
		next:     spec.ID(initial),
		logout:   func() {},
	}
}

// Incremenents the next ID to be used and returns it
func (data *Data) NextID() spec.ID {
	data.next = (data.next + 1) % spec.MaxID
	if data.next == spec.NullID {
		data.next += 1
	}
	return data.next
}

// Whether the connection is logged in or not
func (data *Data) IsLoggedIn() bool {
	return data.user != nil && data.user.User.Username != "" && data.IsConnected()
}

// Whether the connection is or not established
func (data *Data) IsConnected() bool {
	return data.conn != nil
}
