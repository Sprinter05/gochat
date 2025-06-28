package commands

// Contains auxiliary functions that make certain commands work

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
)

/* HELPER FUNCTIONS */

// Requests the user logged in to get its permissions
func GetPermissions(ctx context.Context, cmd Command, uname string) (uint, error) {
	id := cmd.Data.NextID()
	packet, err := spec.NewPacket(
		spec.REQ,
		id,
		spec.EmptyInfo,
		[]byte(uname),
	)
	if err != nil {
		return 0, err
	}

	_, err = cmd.Data.Conn.Write(packet)
	if err != nil {
		return 0, err
	}

	verbosePrint("querying permissions...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.REQ, spec.ERR),
	)
	if err != nil {
		return 0, err
	}

	if reply.HD.Op == spec.ERR {
		return 0, spec.ErrorCodeToError(reply.HD.Info)
	}

	perms, err := spec.BytesToPermission(reply.Args[2])
	if err != nil {
		return 0, err
	}

	return perms, nil
}

// Performs the necessary operations to store a RECIV
// packet in the database (decryption, REQ (if necessary)
// insert...), then returns the decrypted message
func StoreMessage(ctx context.Context, reciv spec.Command, cmd Command) (Message, error) {
	src, err := db.GetUser(
		cmd.Static.DB,
		string(reciv.Args[0]),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if err != nil {
		// The user most likely has not been found, so a REQ is required
		_, reqErr := REQ(ctx, cmd, string(reciv.Args[0]))
		if reqErr != nil {
			return Message{}, reqErr
		}
	}

	prvKey, pemErr := spec.PEMToPrivkey([]byte(cmd.Data.LocalUser.PrvKey))
	if pemErr != nil {
		return Message{}, pemErr
	}

	decrypted, decryptErr := spec.DecryptText(reciv.Args[2], prvKey)
	if decryptErr != nil {
		return Message{}, decryptErr
	}

	stamp, parseErr := spec.BytesToUnixStamp(reciv.Args[1])
	if parseErr != nil {
		return Message{}, parseErr
	}

	_, insertErr := db.StoreMessage(
		cmd.Static.DB,
		src.Username,
		cmd.Data.LocalUser.User.Username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
		string(decrypted),
		stamp,
	)
	if insertErr != nil {
		return Message{}, insertErr
	}

	return Message{
		Sender:    string(reciv.Args[0]),
		Content:   string(decrypted),
		Timestamp: stamp,
	}, nil
}

/* AUXILIARY FUNCTIONS */

// Tries to convert a string into any of the primitive values
func stringToValue(val string, ref reflect.Value) any {
	kind := ref.Kind()

	// Parse as string (returning the value)
	if kind == reflect.String {
		return val
	}

	// Parse as boolean
	if kind == reflect.Bool {
		asBool, err := strconv.ParseBool(val)
		if err == nil {
			return asBool
		}
	}

	// We get the amount of bits if its a numeric
	bits := ref.Type().Bits()

	// Parse as unsigned integer
	if kind >= reflect.Uint && kind <= reflect.Uint64 {
		asUint, err := strconv.ParseUint(val, 10, bits)
		if err == nil {
			// We need this or it will fail when setting
			switch bits {
			case 8:
				return uint8(asUint)
			case 16:
				return uint16(asUint)
			case 32:
				return uint32(asUint)
			}
			return uint(asUint) // Ignore uint64
		}
	}

	// Parse as signed integer
	if kind >= reflect.Int && kind <= reflect.Int64 {
		asInt, err := strconv.ParseInt(val, 10, bits)
		if err == nil {
			// We need this or it will fail when setting
			switch bits {
			case 8:
				return int8(asInt)
			case 16:
				return int16(asInt)
			case 32:
				return int32(asInt)
			}
			return int(asInt) // Ignore int64
		}
	}

	// Parse as floating point number
	if kind >= reflect.Float32 && kind <= reflect.Float64 {
		asFloat, err := strconv.ParseFloat(val, bits)
		if err == nil {
			// We need this or it will fail when setting
			if bits == 32 {
				return float32(asFloat)
			}
			return asFloat
		}
	}

	// If its none of the others then we return nil
	return nil
}

// Tries to log in using a reusable token if applicable
func tokenLogin(ctx context.Context, cmd Command, username string) error {
	id := cmd.Data.NextID()

	token, ok := cmd.Data.GetToken()
	if !ok {
		return ErrorNoReusableToken
	}

	pct, err := spec.NewPacket(
		spec.LOGIN, id,
		spec.EmptyInfo,
		[]byte(username),
		[]byte(token),
	)
	if err != nil {
		return err
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	_, err = cmd.Data.Conn.Write(pct)
	if err != nil {
		return err
	}

	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return err
	}

	if reply.HD.Op == spec.ERR {
		cmd.Data.ClearToken()
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	return nil
}

/* CONFIG FUNCTIONS */

// Sets the value of a configuration struct and returns an error if it failed
// and a rollback function to restore the field to its original value with the
// new value of the field
func setStructConfig(target any, field, value string) (any, func(), error) {
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

	// Get the struct
	objVal := reflect.ValueOf(target).Elem()

	// Apply recursion if necessary
	prefix, suffix, ok := strings.Cut(field, ".")
	if ok {
		// We check if the child field even exists
		_, ok := objType.FieldByName(prefix)
		if !ok {
			return nil, nil, ErrorInvalidField
		}

		// We obtain the value of the child struct
		child := objVal.FieldByName(prefix)
		childPtr := child.Addr().Interface()

		// To apply recursion it must be a struct
		if child.Kind() != reflect.Struct {
			return nil, nil, ErrorInvalidField
		}

		// We pass the pointer to allow modification
		val, rollback, err := setStructConfig(childPtr, suffix, value)
		if err != nil {
			return nil, nil, err
		}

		return val, rollback, nil
	}

	// Check the field's type
	fieldType, ok := objType.FieldByName(field)
	if !ok {
		// Not found in the struct
		return nil, nil, ErrorInvalidField
	}

	// Get the struct value
	fieldVal := objVal.FieldByName(field)

	// Make sure we dont allow modifying foreign keys
	check, ok := fieldType.Tag.Lookup("gorm")
	if ok && strings.Contains(check, "foreignKey") {
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
// parameter can or not be a pointer
func getStructConfig(obj any, prefix string) ([][]byte, error) {
	buf := make([][]byte, 0)

	// Get the type and values about the server struct
	objType := reflect.TypeOf(obj)
	objVal := reflect.ValueOf(obj)

	// We dereference if necessary
	if objType.Kind() == reflect.Pointer {
		objType = objType.Elem()
		objVal = objVal.Elem()
	}

	// We cannot show values if not a struct
	if objType.Kind() != reflect.Struct {
		return nil, ErrorInvalidTarget
	}

	// Show all values of the struct
	for i := 0; i < objType.NumField(); i++ {
		fieldType := objType.Field(i)
		fieldVal := objVal.Field(i)

		// Apply recursion if necessary
		if fieldVal.Kind() == reflect.Struct {
			concat := prefix + "." + fieldType.Name
			recursion, err := getStructConfig(fieldVal.Interface(), concat)
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

	return buf, nil
}

/* PRINTING FUNCTIONS */

// Prints out all local users on the current server and
// returns an array with its usernames.
func printServerLocalUsers(cmd Command) ([][]byte, error) {
	localUsers, err := db.GetServerLocalUsers(
		cmd.Static.DB,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)

	if err != nil {
		return [][]byte{}, err
	}

	users := make([][]byte, 0, len(localUsers))
	cmd.Output(fmt.Sprintf("local users from %s - %s:%d:",
		cmd.Data.Server.Name,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port),
		USRSRESPONSE,
	)

	for _, v := range localUsers {
		users = append(users, []byte(v.User.Username))
		cmd.Output(v.User.Username, USRSRESPONSE)
	}

	return users, nil
}

// Prints out all external users on the current server and
// returns an array with its usernames.
func printExternalUsers(cmd Command) ([][]byte, error) {
	externalUsers, err := db.GetRequestedUsers(cmd.Static.DB)

	if err != nil {
		return [][]byte{}, err
	}

	users := make([][]byte, 0, len(externalUsers))
	cmd.Output("all external users:", USRSRESPONSE)

	for _, v := range externalUsers {
		users = append(users, []byte(v.User.Username))
		cmd.Output(fmt.Sprintf("%s (%s - %s:%d)",
			v.User.Username,
			v.User.Server.Name,
			v.User.Server.Address,
			v.User.Server.Port),
			USRSRESPONSE,
		)
	}

	return users, nil
}

// Prints out all local users on the current server and
// returns an array with its usernames.
func printAllLocalUsers(cmd Command) ([][]byte, error) {
	localUsers, err := db.GetAllLocalUsers(
		cmd.Static.DB,
	)

	if err != nil {
		return [][]byte{}, err
	}

	users := make([][]byte, 0, len(localUsers))
	cmd.Output("all local users:", USRSRESPONSE)

	for _, v := range localUsers {
		addr := "(Unknown)"
		if v.User.Server.Port != 0 {
			addr = fmt.Sprintf(
				"(%s - %s:%d)",
				v.User.Server.Name,
				v.User.Server.Address,
				v.User.Server.Port,
			)
		}

		str := fmt.Sprintf(
			"%s %s",
			v.User.Username,
			addr,
		)
		users = append(users, []byte(str))
		cmd.Output(str, USRSRESPONSE)
	}

	return users, nil
}

// Prints a packet.
func packetPrint(pct []byte, cmd Command) {
	pctCmd := spec.ParsePacket(pct)
	str := fmt.Sprintf(
		"Client packet to be sent:\n%s",
		pctCmd.Contents(),
	)
	cmd.Output(str, PACKET)
}

// Prints text if the verbose mode is on.
func verbosePrint(text string, args Command) {
	if args.Static.Verbose {
		args.Output(text, INTERMEDIATE)
	}
}

/* WAITLIST FUNCTIONS */

// Returns a function that returns true if the received command fulfills
// the given conditions in the arguments (ID and operations).
// This is used to dinamically create functions that retrieve commands
// from the waitlist with waitlist.Get()
func Find(id spec.ID, ops ...spec.Action) func(cmd spec.Command) bool {
	return func(cmd spec.Command) bool {
		if cmd.HD.ID == id && slices.Contains(ops, cmd.HD.Op) {
			return true
		}

		return false
	}
}

// Returns an appropiate waitlist
func DefaultWaitlist() models.Waitlist[spec.Command] {
	return models.NewWaitlist(0, func(a spec.Command, b spec.Command) int {
		switch {
		case a.HD.ID > b.HD.ID:
			return 1
		case a.HD.ID < b.HD.ID:
			return -1
		default:
			return 0
		}
	})
}
