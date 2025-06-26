package commands

import (
	"context"
	"fmt"
	mrand "math/rand/v2"
	"net"
	"reflect"
	"strings"
	"sync"

	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
	"gorm.io/gorm"
)

/* DATA */

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

/* DATA FUNCTIONS */

// Gets a reusable token if it exists
func (d *Data) GetToken() (string, bool) {
	d.mut.RLock()
	defer d.mut.RUnlock()
	return d.token, d.token != ""
}

// Sets a new reusable token
func (d *Data) SetToken(t string) {
	d.mut.Lock()
	defer d.mut.Unlock()
	d.token = t
}

// Empties the reusabke token
func (d *Data) ClearToken() {
	d.mut.Lock()
	defer d.mut.Unlock()
	d.token = ""
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

/* CONFIG */

// Represents a function to update an object on the database
type ConfigUpdate func(db *gorm.DB, obj any, column string, val any) error

// Represents a config struct to be passed to modify
type ConfigObj struct {
	Prefix       string       // Should start with capital letter
	Object       any          // Can be or not a pointer (read the function precondition)
	Precondition func() error // Condition needed for it to be able to set a value
	Update       ConfigUpdate // Called to update values on the database
	Finish       func()       // Run a function once it has been updated
}

/* AUXILIARY CONFIG FUNCTIONS */

// Gets the child object on a struct. It is addressable.
func getStructChild(prefix string, objVal reflect.Value) (reflect.Value, bool) {
	objType := objVal.Type()
	zero := reflect.Zero(objType)

	// Check that its a struct
	if objVal.Kind() != reflect.Struct {
		return zero, false
	}

	// We check if the child field even exists
	fieldType, ok := objType.FieldByName(prefix)
	if !ok {
		return zero, false
	}

	// Make sure we dont allow modifying foreign keys
	check, ok := fieldType.Tag.Lookup("gorm")
	if ok && strings.Contains(check, "foreignKey") {
		return zero, false
	}

	// We obtain the value of the child struct
	childVal := objVal.FieldByName(prefix)
	return childVal, true
}

// Assumes the value passed is a struct (dereferenced)
func loopGetStruct(objType reflect.Type, objVal reflect.Value, prefix string) [][]byte {
	// Preallocation
	len := objType.NumField()
	buf := make([][]byte, 0, len)

	for i := 0; i < objType.NumField(); i++ {
		fieldType := objType.Field(i)
		fieldVal := objVal.Field(i)

		// Apply recursion if necessary
		if fieldVal.Kind() == reflect.Struct {
			concat := prefix + "." + fieldType.Name
			recursion, err := getConfig(fieldVal.Interface(), concat)
			if err == nil {
				buf = append(buf, recursion...)
			}

			continue
		}

		// We do not show internal IDs
		if strings.Contains(fieldType.Name, "ID") {
			continue
		}

		// We also skip foreign keys
		check, ok := fieldType.Tag.Lookup("gorm")
		if ok && strings.Contains(check, "foreignKey") {
			continue
		}

		// Add the information to the list
		str := fmt.Sprintf("%s.%s = %v", prefix, fieldType.Name, fieldVal)
		buf = append(buf, []byte(str))
	}

	return buf
}

/* CONFIG FUNCTIONS */

// Sets the value of a configuration struct and returns an error if it failed
// and a rollback function to restore the field to its original value with the
// new value of the field.
func setConfig(target any, field, value string) (any, func(), error) {
	// Allowing ID modification would be too dangerous
	if strings.Contains(field, "ID") {
		return nil, nil, ErrorInvalidField
	}

	// Make sure we are given a pointer
	objPtr := reflect.TypeOf(target)
	if objPtr.Kind() != reflect.Pointer {
		return nil, nil, ErrorInvalidTarget
	}

	// Make sure what we dereference is a struct
	objType := objPtr.Elem()
	if objType.Kind() != reflect.Struct {
		return nil, nil, ErrorInvalidTarget
	}

	// Get the object
	objVal := reflect.ValueOf(target).Elem()

	// Apply recursion if necessary
	prefix, suffix, ok := strings.Cut(field, ".")
	if ok {
		// We get the child
		child, ok := getStructChild(prefix, objVal)
		if !ok {
			return nil, nil, ErrorInvalidField
		}

		// Get its address
		childPtr := child.Addr().Interface()

		// To apply recursion it must be a struct
		if child.Kind() != reflect.Struct {
			return nil, nil, ErrorInvalidField
		}

		// We pass the pointer to allow modification
		val, rollback, err := setConfig(childPtr, suffix, value)
		if err != nil {
			return nil, nil, err
		}

		return val, rollback, nil
	}

	// Attempt to get the field
	fieldVal, ok := getStructChild(prefix, objVal)
	if !ok {
		return nil, nil, ErrorInvalidField
	}

	// Not settable
	if !fieldVal.CanSet() {
		return nil, nil, ErrorCannotSet
	}

	// Get the value from the string
	val := stringToValue(value, fieldVal)
	if val == nil {
		return nil, nil, ErrorCannotSet
	}

	ref := reflect.ValueOf(val)

	// Used to rollback
	tmp := fieldVal.Interface()
	rollback := func() {
		fieldVal.Set(reflect.ValueOf(tmp))
	}

	// This check is necessary to avoid panics
	if ref.Kind() != fieldVal.Kind() {
		return nil, nil, ErrorCannotSet
	}

	// We set the value and return
	fieldVal.Set(ref)
	return val, rollback, nil
}

// Gets the config parameters of a struct and a boolean indicating
// if it was possible to retrieve the configuration. The passed
// parameter can or not be a pointer.
func getConfig(obj any, prefix string) ([][]byte, error) {
	buf := make([][]byte, 0)

	// Get the type and values about the object
	objType := reflect.TypeOf(obj)
	objVal := reflect.ValueOf(obj)

	// We dereference if necessary
	if objType.Kind() == reflect.Pointer {
		objType = objType.Elem()
		objVal = objVal.Elem()
	}

	// Loop for structs
	if objType.Kind() == reflect.Struct {
		rec := loopGetStruct(objType, objVal, prefix)
		buf = append(buf, rec...)
		return buf, nil
	}

	// Not applicable
	return nil, ErrorInvalidTarget
}
