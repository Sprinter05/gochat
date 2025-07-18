package ui

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
)

/* CONTEXTS */

// Defines the parent context used for any event
// to be cancelled throughout a connection.
type Connection struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Sets a new context creating it from the given one
// by cancelling the previous one first
func (c *Connection) Create(background context.Context) {
	c.Cancel()
	ctx, cancel := context.WithCancel(background)
	c.ctx = ctx
	c.cancel = cancel
}

// Gets the current context
func (c *Connection) Get() context.Context {
	return c.ctx
}

// Cancels the current context and sets it to the default one
func (c *Connection) Cancel() {
	c.cancel()
	c.ctx = context.Background()
}

// Returns a new timeout using the parent context
func timeout(s Server, data *cmds.Data) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(
		s.Context().Get(),
		time.Duration(cmdTimeout)*time.Second,
	)

	if data != nil {
		go data.Waitlist.Timeout(ctx)
	}

	return ctx, cancel
}

/* INTERFACE */

// Identifies the source used by gochat
type Source struct {
	Address string
	Port    uint16
}

// The network will always be "tcp"
func (s Source) Network() string {
	return "tcp"
}

// Returns the address and port of the server separated by a colon
func (s Source) String() string {
	return fmt.Sprintf("%s:%d", s.Address, s.Port)
}

// Identifies the operations a server
// must fulfill in order to be considered
// a server by the TUI.
type Server interface {
	// Updates the values of the server according to the database
	Update()

	// Returns the name of the server and if it is secure
	Name() string

	// Returns the address corresponding to their endpoint
	// and a boolean indicating if the address is valid
	Source() net.Addr

	// Returns all messages contained in the specified buffer
	Messages(string) []Message

	// Returns all notifications belonging to the server
	Notifications() Notifications

	// Tries to receive a message and indicates if it was for them
	// and if any error occurred
	Receive(Message) (bool, error)

	// Returns the internal buffer struct they may contain
	Buffers() *Buffers

	// Returns the context of the connection
	Context() *Connection

	// Returns the command asocciated data and whether
	// they are connected to the endpoint or not
	Online() (*cmds.Data, bool)
}

// Returns the currently active server.
func (t *TUI) Active() Server {
	s, ok := t.servers.Get(t.focus)
	if !ok {
		// This condition should never trigger
		panic("active server does not exist")
	}

	return s
}

/* RENDERING */

// Adds a server connected to a remote endpoint, stores it in
// the database, adds it to the TUI but does not changes to it.
func (t *TUI) addServer(name string, addr string, port uint16, tls bool) error {
	if t.servers.Len() >= int(maxServers) {
		return ErrorMaxServers
	}

	_, ok := t.servers.Get(name)
	if ok {
		return ErrorExists
	}

	source := Source{
		Address: addr,
		Port:    port,
	}

	if t.existsServer(source) {
		return ErrorExists
	}

	s := NewRemoteServer(name, source)

	serv, err := db.AddServer(
		t.db,
		addr,
		port,
		name,
		tls,
	)
	if err != nil {
		return err
	}
	s.data.Server = &serv

	t.servers.Add(name, s)
	num := t.servers.Len()

	// We check if there are any available indexes for the server
	indexes := len(t.status.serverIndexes)
	if indexes > 0 {
		num = t.status.serverIndexes[0] + 1
		t.status.serverIndexes = t.status.serverIndexes[1:]
	}

	t.comp.servers.AddItem(name, tlsText(source, tls), ascii(num), nil)
	t.renderServer(name)
	return nil
}

// Adds a server from the database that already existed
func (t *TUI) showServer(name string) error {
	serv, err := db.GetServerByName(t.db, name)
	if err != nil {
		return err
	}

	source := Source{
		Address: serv.Address,
		Port:    serv.Port,
	}

	s := NewRemoteServer(name, source)
	s.data.Server = &serv

	t.servers.Add(name, s)
	num := t.servers.Len()
	indexes := len(t.status.serverIndexes)
	if indexes > 0 {
		num = t.status.serverIndexes[0] + 1                 // FIFO
		t.status.serverIndexes = t.status.serverIndexes[1:] // Remove
	}

	t.comp.servers.AddItem(name, tlsText(source, serv.TLS), ascii(num), nil)

	t.renderServer(name)
	return nil
}

// Finds a server by a given address
func (t *TUI) existsServer(addr net.Addr) bool {
	list := t.servers.GetAll()
	for _, v := range list {
		source := v.Source()
		if source == nil {
			continue
		}

		if addr.String() == source.String() {
			return true
		}
	}

	return false
}

// Changes the TUI component according to its
// internal index and renders the server
func (t *TUI) changeServer(i int) {
	if i < 0 || i >= t.comp.servers.GetItemCount() {
		return
	}

	t.comp.servers.SetCurrentItem(i)
	text, _ := t.comp.servers.GetItemText(i)
	t.renderServer(text)
}

// Finds a server by a given name and returns its internal
// index and whether it was found or not.
func (t *TUI) findServer(name string) (int, bool) {
	l := t.comp.servers.FindItems(name, "", false, false)

	if len(l) != 0 {
		return l[0], true
	}

	return -1, false
}

// Deletes a server and all its contents (except in the database).
// It then changes to the "Local" server by default. This also
// implies the "Local" server cannot be hidden.
func (t *TUI) hideServer(name string) {
	s, ok := t.servers.Get(name)
	if !ok {
		return
	}

	_, chk := s.(*LocalServer)
	if chk {
		t.showError(ErrorLocalServer)
		return
	}

	i, ok := t.findServer(name)
	if ok {
		t.comp.servers.RemoveItem(i)
	}
	t.status.serverIndexes = append(t.status.serverIndexes, i)

	// Cleanup resources and wait a bit
	data, connected := s.Online()
	if connected {
		err := cmds.DISCN(
			cmds.Command{
				Output: t.systemMessage(),
				Data:   data,
				Static: t.static(),
			},
		)

		if err != nil {
			t.showError(err)
			return
		}
	}

	// Gives time for deletion to happen
	<-time.After(100 * time.Millisecond)

	t.servers.Remove(name)

	t.renderServer(localServer)
}

// Removes a server from the database. This assumes
// the server has already been removed from the TUI.
func (t *TUI) removeServer(s Server) {
	_, chk := s.(*LocalServer)
	if chk {
		t.showError(ErrorLocalServer)
		return
	}

	addr := s.Source()
	source, ok := addr.(Source)
	if !ok {
		t.showError(ErrorInvalidAddress)
		return
	}

	db.RemoveServer(t.db, source.Address, source.Port)
}

// Changes to a server specified by its name and updates all
// TUI components accordingly. It also renders the last
// buffer that was in use.
func (t *TUI) renderServer(name string) {
	s, ok := t.servers.Get(name)
	if !ok {
		return
	}

	t.focus = name

	index, ok := t.findServer(name)
	if ok {
		t.comp.servers.SetCurrentItem(index)
	}

	data, online := s.Online()
	if online {
		t.comp.servers.SetSelectedTextColor(tcell.ColorGreen)
		if data.IsLoggedIn() {
			uname := data.LocalUser.User.Username
			t.comp.input.SetLabel(unameLabel(uname))
		} else {
			t.comp.input.SetLabel(defaultLabel)
		}
	} else {
		t.comp.servers.SetSelectedTextColor(tcell.ColorPurple)
		t.comp.input.SetLabel(defaultLabel)
	}

	empty := func(string, cmds.OutputType) {}
	updateOnlineUsers(t, s, empty)

	t.comp.buffers.Clear()
	if s.Buffers().tabs.Len() == 0 {
		t.comp.text.Clear()
		return
	}

	// Sort buffers before showing them
	tabs := s.Buffers().tabs.GetAll()
	slices.SortFunc(tabs, func(a, b *tab) int {
		if a.creation < b.creation {
			return -1
		} else if a.creation > b.creation {
			return 1
		}

		return 0 // Equal
	})

	for _, v := range tabs {
		if v.index != -1 {
			t.comp.buffers.AddItem(v.name, "", ascii(v.index), nil)
		}
	}

	i, ok := t.findBuffer(t.Buffer())
	if !ok {
		panic("cannot open server buffer on change")
	}

	t.changeBuffer(i)
}

/* REMOTE SERVER */

// Specifies a remote server
type RemoteServer struct {
	addr Source // Implements net.Addr
	name string // Name of the server

	conn Connection // Used for context propagation

	bufs   Buffers                    // Buffer data
	data   cmds.Data                  // Commands data
	notifs models.Table[string, uint] // Notifications
}

// Creates a new empty remote server with the given data
func NewRemoteServer(name string, addr Source) *RemoteServer {
	return &RemoteServer{
		addr: addr,
		name: name,
		conn: Connection{
			ctx:    context.Background(),
			cancel: func() {},
		},
		bufs: Buffers{
			tabs: models.NewTable[string, *tab](0),
		},
		data:   cmds.NewEmptyData(),
		notifs: models.NewTable[string, uint](0),
	}
}

func (s *RemoteServer) Messages(name string) []Message {
	t, ok := s.bufs.tabs.Get(name)
	if !ok {
		return nil
	}

	msgs := t.messages.Copy(0)
	slices.SortFunc(msgs, func(a, b Message) int {
		if a.Timestamp.Before(b.Timestamp) {
			return -1
		} else if a.Timestamp.After(b.Timestamp) {
			return 1
		}

		return 0
	})

	return msgs
}

func (s *RemoteServer) Receive(msg Message) (bool, error) {
	if msg.Source != s.name {
		// Not this destination
		return false, nil
	}

	check := strings.ReplaceAll(msg.Content, "\n", "")
	if check == "" {
		// Empty content
		return false, ErrorNoText
	}

	b, ok := s.bufs.tabs.Get(msg.Buffer)
	if !ok {
		return false, nil
	}

	b.messages.Add(msg)
	return true, nil
}

func (s *RemoteServer) Buffers() *Buffers {
	return &s.bufs
}

func (s *RemoteServer) Online() (*cmds.Data, bool) {
	return &s.data, s.data.IsConnected()
}

func (s *RemoteServer) Source() net.Addr {
	return s.addr
}

func (s *RemoteServer) Name() string {
	return s.name
}

func (s *RemoteServer) Context() *Connection {
	return &s.conn
}

func (s *RemoteServer) Notifications() Notifications {
	return Notifications{
		data: &s.notifs,
	}
}

func (s *RemoteServer) Update() {
	s.name = s.data.Server.Name
	s.addr = Source{
		Address: s.data.Server.Address,
		Port:    s.data.Server.Port,
	}
}

/* LOCAL SERVER */

// Specifies a local server that is not connected
// to any remote endpoint
type LocalServer struct {
	name string  // Name of the server
	bufs Buffers // Buffer data
}

func (l *LocalServer) Messages(name string) []Message {
	var ret []Message
	t, ok := l.bufs.tabs.Get(name)
	if !ok {
		return nil
	}
	msgs := t.messages.Copy(0)

	logo := Message{
		Buffer:  systemBuffer,
		Content: Logo[1:],
	}

	ret = append(ret, logo)
	ret = append(ret, msgs...)

	return ret
}

func (l *LocalServer) Receive(msg Message) (bool, error) {
	// Only local server should be nil
	if msg.Source != l.name {
		// Not for this server
		return false, nil
	}

	b, ok := l.bufs.tabs.Get(msg.Buffer)
	if !ok {
		// Not for this server
		return false, nil
	}

	if b.system && msg.Sender == selfSender {
		return false, ErrorSystemBuf
	}

	b.messages.Add(msg)
	return true, nil
}

func (l *LocalServer) Buffers() *Buffers {
	return &l.bufs
}

func (l *LocalServer) Source() net.Addr {
	return nil
}

func (l *LocalServer) Online() (*cmds.Data, bool) {
	return nil, false
}

func (l *LocalServer) Name() string {
	return l.name
}

func (l *LocalServer) Context() *Connection {
	return nil
}

func (l *LocalServer) Notifications() Notifications {
	return Notifications{
		data: nil,
	}
}

func (l *LocalServer) Update() {}
