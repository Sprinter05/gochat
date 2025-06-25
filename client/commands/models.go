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
	Conn     net.Conn                      // Specifies the connection to the server
	Logout   context.CancelFunc            // Specifies the function to call on a logout for context propagation
	Waitlist models.Waitlist[spec.Command] // Stores all packets to be retrieved later

	// Using pointers so that "nil" can be used
	Server    *db.Server    // Specifies the database server
	LocalUser *db.LocalUser // Specifies the logged in user

	token string  // Reusable token in case of TLS usage
	next  spec.ID // Specifies the next ID that should be used when sending a packet

	mut sync.RWMutex // Specifies the mutex protecting token and next
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

func (d *Data) GetToken() string {
	d.mut.RLock()
	defer d.mut.RUnlock()
	return d.token
}

func (d *Data) SetToken(t string) {
	d.mut.Lock()
	defer d.mut.Unlock()
	d.token = t
}

// Creates a new empty but initialised struct for Data
func NewEmptyData() Data {
	initial := mrand.IntN(int(spec.MaxID))

	return Data{
		Waitlist: DefaultWaitlist(),
		Logout:   func() {},
		next:     spec.ID(initial),
	}
}

// Incremenents the next ID to be used and returns it
func (data *Data) NextID() spec.ID {
	data.mut.Lock()
	defer data.mut.Unlock()
	data.next = (data.next + 1) % spec.MaxID
	if data.next == spec.NullID {
		data.next += 1
	}
	return data.next
}

// Whether the connection is logged in or not
func (data *Data) IsLoggedIn() bool {
	return data.LocalUser != nil && data.LocalUser.User.Username != "" && data.IsConnected()
}

// Whether the connection is or not established
func (data *Data) IsConnected() bool {
	return data.Conn != nil
}
