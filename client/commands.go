package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/Sprinter05/gochat/internal/spec"
)

// Interface for all commands
// ! Golang esta pensando para simplificar todo lo maximo posible
// ! Solo usa interfaces si es necesario, no lo fuerces
// ! A mi esto me parece forzar el que use una interfaz cuando no hace falta
type CommandFunc interface {
	Run(cmd *spec.Command, nArg uint8, db *sql.DB) error
}

// Type for commands with arguments
// ! Las funciones de comandos deberian recibir copias no punteros
// ! Siempre elige copia a pointer como argumento salvo que lo vayas a modificar para uso de otra función
type CmdArgs func(cmd *spec.Command, nArg uint8) error

func (command CmdArgs) Run(cmd *spec.Command, nArg uint8, db *sql.DB) error {
	return command(cmd, nArg)
}

// Type for commands with arguments and database interaction
type CmdArgsDB func(cmd *spec.Command, nArg uint8, db *sql.DB) error

func (command CmdArgsDB) Run(cmd *spec.Command, nArg uint8, db *sql.DB) error {
	return command(cmd, nArg, db)
}

// Type for commands with no arguments
// ! Realmente necesitas dos tipos para comandos con y sin argumentos?
// ! Recomendaria usar simplemente una variable que comprueba los args y pista
type CmdNoArgs func() error

func (command CmdNoArgs) Run(cmd *spec.Command, nArg uint8, db *sql.DB) error {
	return command()
}

// Map with all client commands except EXIT.
// TODO: Some functions shall be moved to the higher-level shell
// ! No pongas mapas globales en mayuscula pq asi lo exportas y no deberias exportar un mapa ya que no es inmutable
var clientCmds = map[string]CommandFunc{
	"VER":        CmdNoArgs(ver),
	"HELP":       CmdNoArgs(help),
	"VERBOSE":    CmdNoArgs(verbose),
	"PENDING":    CmdNoArgs(printPending),
	"CREATEUSER": CmdArgsDB(createUser),
	"REGUSER":    CmdNoArgs(regUser),
	"REG":        CmdArgs(sendPacket),
	"LOGIN":      CmdArgs(sendPacket),
	"VERIF":      CmdArgs(sendPacket),
	"REQ":        CmdArgs(sendPacket),
	"USRS":       CmdArgs(usrs),
	"MSG":        CmdArgsDB(sendMSG),
	"RECIV":      CmdArgs(sendPacket),
	"LOGOUT":     CmdArgs(sendPacket),
	"DEREG":      CmdArgs(sendPacket),
}

// Map that associates the number of arguments required for each command.
var NumArgs = map[string]uint8{
	"VER":        0,
	"HELP":       0,
	"VERBOSE":    0,
	"PENDING":    0,
	"LOADUSER":   1,
	"CREATEUSER": 1,
	"REGUSER":    2,
	"REG":        2,
	"LOGIN":      1,
	"VERIF":      1,
	"REQ":        1,
	"USRS":       1,
	"MSG":        3,
	"RECIV":      0,
	"LOGOUT":     0,
	"DEREG":      0,
}

// Map with all server commands
// ! Usa una funcion que devuelva los valores del mapa en vez de usar un mapa a pelo
var serverCmds = map[spec.Action]func(*spec.Command) error{
	spec.OK:    AcknowledgeReply,
	spec.ERR:   PrintError,
	spec.VERIF: DecryptVERIF,
	spec.REQ:   StoreRequestedUser,
	spec.USRS:  PrintUSRS,
	spec.RECIV: StoreDecypheredMessage,
}

// CLIENT COMMANDS

// ! 90% de estas funciones sobra, son literalmente funciones de 1 linea
// ! Usar funciones de 1-2 lineas solo causa mas lentitud llamando funciones que se podrian ahorrar

// Execution code of the VER command
// ! Sobretodo en este caso que ni siquiera devuelve nunca un error
func ver() error {
	fmt.Printf("gochat version %d\n", spec.ProtocolVersion)

	return nil
}

// Execution code of the HELP command
// ! Sobretodo en este caso que ni siquiera devuelve nunca un error
func help() error {
	fmt.Println(helpText)

	return nil
}

// Execution code of the VERBOSE command
// ! Sobretodo en este caso que ni siquiera devuelve nunca un error
func verbose() error {
	IsVerbose = !IsVerbose
	if IsVerbose {
		fmt.Println("Verbose mode turned on")
	} else {
		fmt.Println("Verbose mode turned off")
	}

	return nil
}

// Prints the commands that are yet to receive a response
// ! Sobretodo en este caso que ni siquiera devuelve nunca un error
func printPending() error {
	// Checks the number of pending packets
	if len(PendingBuffer) == 0 {
		fmt.Printf("There are no pending packets\n")
	} else {
		fmt.Printf("Pending packets:\n----------------\n")
		for v, i := range PendingBuffer {
			fmt.Printf("ID %d: Action code: %d (%s)\n", v, i, spec.CodeToString(spec.Action(uint8(i))))
		}
	}

	return nil
}

// Creates a user with a username received as input
func createUser(cmd *spec.Command, nArg uint8, db *sql.DB) error {
	// Checks argument count
	if cmd.HD.Args != nArg {
		return fmt.Errorf("%s: Incorrect number of arguments", spec.CodeToString(cmd.HD.Op))
	}
	user, createErr := NewUser(string(cmd.Args[0]), db)
	if createErr != nil {

		return createErr
	}
	fmt.Print("User " + user.username + " created successfully\n")
	CurUser = *user

	return nil
}

// Sends a REG package for the current user to the server automatically
// TODO: Improve this
func regUser() error {
	// ! Usa un booleano que simplemente compruebe la validez del usuario
	// ! Comparar structs enteros a si a pelo no es recomendable.
	if (Client{}) == CurUser {
		return fmt.Errorf("error: there is no current user logged in")
	}

	pubKey, keyErr := spec.PubkeytoPEM(&CurUser.keyPair.PublicKey)
	if keyErr != nil {

		return keyErr
	}

	// ! Usa make y append o copy
	args := [][]byte{[]byte(CurUser.username), pubKey}

	// ! Todo esto te lo ahorras usando NewPacket() del internal/spec

	payloadLen := 0
	for _, arg := range args {
		payloadLen += len(arg) + 2 // + 2 to include the CRLF in each argument
	}

	// Creates header
	header := spec.Header{
		Ver:  spec.ProtocolVersion,
		Op:   spec.REG,
		Info: spec.EmptyInfo,
		Args: NumArgs["REGUSER"], // ! Usa una funcion que compruebe el valor no acceder directamente a la tabla
		Len:  uint16(payloadLen),
		ID:   spec.ID(spec.GeneratePacketID(PendingBuffer)),
	}
	// Creates command
	cmd := spec.Command{HD: header, Args: args}
	sendErr := sendPacket(&cmd, NumArgs["REGUSER"])
	return sendErr
}

// Execution code of the MSG command (requires database insert and message encryption)
func sendMSG(cmd *spec.Command, nArg uint8, db *sql.DB) error {
	// Checks argument count
	if cmd.HD.Args != nArg {
		return fmt.Errorf("%s: Incorrect number of arguments", spec.CodeToString(cmd.HD.Op))
	}
	// Stores the message in plain text to be stored in the database later
	// ! Esto es raro que funcione pq abajo estas sobreescribiendo cmd.Args[2]
	// ! plainMessage es un slice aka pointer por tanto es posible que se sobreescriba abajo
	// ! Usa make y copy preferiblemente
	plainMessage := cmd.Args[2]

	// The packet message is taken and is encrypted
	var encryptErr error

	pem, _ := GetUserPubkey(string(cmd.Args[0]), db)
	destPubKey, _ := spec.PEMToPubkey(pem)
	cmd.Args[2], encryptErr = spec.EncryptText(cmd.Args[2], destPubKey)
	if encryptErr != nil {
		return encryptErr
	}

	// Packet is sent
	sendErr := sendPacket(cmd, nArg)
	if sendErr != nil {
		return sendErr
	}
	// If the message is sent correctly, then it is also stored in the database
	// Casts the received timestamp
	// ! Usa las funciones del paquete time no parseInt
	stamp, _ := strconv.ParseInt(string(cmd.Args[1]), 10, 64)
	destination_username := cmd.Args[0]
	dbErr := AddMessage(CurUser.username, string(destination_username), stamp, string(plainMessage), db)

	return dbErr
}

// Rearranges a packet to send a USRS packet
func usrs(cmd *spec.Command, nArg uint8) error {
	// Checks argument count
	if cmd.HD.Args != nArg {
		return fmt.Errorf("%s: Incorrect number of arguments", spec.CodeToString(cmd.HD.Op))
	}
	var convErr error
	// Moves the argument (which contains the USRS option) to the Info field of the header
	infoVal, convErr := strconv.Atoi(string(cmd.Args[0]))
	if convErr != nil {

		return convErr
	}
	if !(infoVal == 0 || infoVal == 1) {

		return fmt.Errorf("error: USRS argument should be 0 (all users) or 1 (online users)")
	}
	cmd.HD.Info = uint8(infoVal)
	// Initializes the argument slice to remove arguments
	cmd.Args = make([][]byte, 0) //! Si le das capacidad 0 no estas prealocando nada y vas a ver 0 beneficio
	// Sends the rearranged packet
	sendErr := sendPacket(cmd, nArg)
	if sendErr != nil {
		return sendErr
	}
	printCmdIfVerbose(*cmd)
	return nil
}

// Generic function able to execute packet-sending commands.
func sendPacket(cmd *spec.Command, nArg uint8) error {
	// Checks argument count
	// ! Comprueba aqui usando el mapa con los argumentos en vez de ir pasando nArg por todos lados
	if cmd.HD.Args != nArg {
		return fmt.Errorf("%s: Incorrect number of arguments", spec.CodeToString(cmd.HD.Op))
	}
	// Creates packet with the proper headers
	pct, err := spec.NewPacket(cmd.HD.Op, cmd.HD.ID, cmd.HD.Info, cmd.Args...)
	if err != nil {

		return fmt.Errorf("%s: %s", spec.CodeToString(cmd.HD.Op), err)
	}
	// Sends packet to server
	_, errW := gCon.Write(pct)
	if errW != nil {
		return fmt.Errorf("%s: Unable to write packet to connection", spec.CodeToString(cmd.HD.Op))
	}
	printCmdIfVerbose(*cmd)
	// If the packet is sent correctly, it is added to PendingBuffer
	// ! No uses el mapa directamente a pelo, usa funciones que accedan al mapa exclusivamente
	// ! Y que pending buffer use un mutex para ser seguro concurrentemente
	PendingBuffer[uint16(cmd.HD.ID)] = uint8(cmd.HD.Op)
	return nil
}

// SERVER COMMANDS

// Acknowledges a reply, removing the packet with the reply's ID from the pending packet buffer.
// NOTE: Some server command functions contain AcknowledgeReply because some server commands do not
// send an OK packet
func AcknowledgeReply(pct *spec.Command) error {
	_, ok := PendingBuffer[uint16(pct.HD.ID)]
	if ok {
		// Deletes the ID of the packet that was waiting for the now received reply
		delete(PendingBuffer, uint16(pct.HD.ID))
		if IsVerbose {
			ClearPrompt()
			fmt.Printf("Packet with ID %d has been acknowledged and removed from the buffer\n", pct.HD.ID)
		}
		return nil
	} else if pct.HD.ID == 0 {
		// The packet is autonomous and doesn't require to acknoledge a reply
		return nil
	}
	return fmt.Errorf("error: Packet with ID %d and action %s has been received but was not expected", pct.HD.ID, spec.CodeToString(pct.HD.Op))
}

// Prints the received error in the shell
// ! Esta funcion sobra, son 2 lineas
func PrintError(pct *spec.Command) error {
	// Prints error package information
	fmt.Printf("An error packet has been received with ID %d and information code %d (%s)\n", pct.HD.ID, pct.HD.Info, spec.ErrorCodeToError(pct.HD.Info).Error())
	// Removes ID from buffer
	ackErr := AcknowledgeReply(pct)

	return ackErr
}

// Replaces the encrypted argument with its decrypted version
func DecryptVERIF(pct *spec.Command) error {
	AcknowledgeReply(pct)
	if (Client{}) == CurUser {
		return fmt.Errorf("error: cannot decrypt message as there is no logged in user in the shell")
	}
	encrypted := []byte(pct.Args[0])
	// Declares the error early because decrypted is already an existing variable
	var err error
	// Decrypts the received argument
	pct.Args[0], err = spec.DecryptText(encrypted, CurUser.keyPair)

	// TODO: TEMPORARY
	// ! Usa make y append/copy
	args := [][]byte{[]byte(CurUser.username), pct.Args[0]}
	payloadLen := 0
	for _, arg := range args {
		payloadLen += len(arg) + 2 // + 2 to include the CRLF in each argument
	}
	// Creates header
	// ! Usa NewPacket() y te ahorras todo esto que ademas es casi duplicado de arriba
	header := spec.Header{
		Ver:  spec.ProtocolVersion,
		Op:   spec.VERIF,
		Info: spec.EmptyInfo,
		Args: NumArgs["VERIF"],
		Len:  uint16(payloadLen),
		ID:   spec.ID(spec.GeneratePacketID(PendingBuffer)),
	}
	// Creates command
	cmd := spec.Command{HD: header, Args: args}
	sendPacket(&cmd, NumArgs["VERIF"])
	return err
}

// Stores in the client database the received public key along with the username it belongs to
// ! No uses pointer receiver, usa una copia
func StoreRequestedUser(pct *spec.Command, db *sql.DB) error {
	AcknowledgeReply(pct)
	username := string(pct.Args[0])
	pkey := pct.Args[1]
	dbErr := AddUser(username, string(pkey), db)
	return dbErr
}

// Prints the users provided by the received USRS response
// ! No uses pointer receiver, usa una copia
// ! Esta funcion sobra son 2 lineas
func PrintUSRS(pct *spec.Command) error {
	AcknowledgeReply(pct)
	fmt.Println(string(pct.Args[0]))
	return nil
}

// Decyphers the received message in the packet and stores it in the client database
func StoreDecypheredMessage(pct *spec.Command, db *sql.DB) error {
	source_username := string(pct.Args[0])
	// ! Porque ignoras el error?
	stamp, _ := spec.BytesToUnixStamp(pct.Args[1])
	// Decrypts the message
	encrypted := pct.Args[2]
	fmt.Println(encrypted)
	decrypted, decryptErr := spec.DecryptText(encrypted, CurUser.keyPair)
	if decryptErr != nil {
		return decryptErr
	}
	dbErr := AddMessage(source_username, CurUser.username, stamp.Unix(), string(decrypted), db)
	return dbErr
}

// ! Esta funcion sobra son 3 lineas
func printCmdIfVerbose(cmd spec.Command) {
	if IsVerbose {
		fmt.Println("The following command has been sent:")
		cmd.Print()
	}
}
