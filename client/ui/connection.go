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

// Sets a new context by cancelling the previous one first
func (c *Connection) Set(background context.Context) {
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
func timeout(s Server) (context.Context, context.CancelFunc) {
	return context.WithTimeout(
		s.Connection().Get(),
		time.Duration(cmdTimeout)*time.Second,
	)
}

/* INTERFACE */

// Identifies the operations a server
// must fulfill in order to be considered
// a server by the TUI.
type Server interface {
	// Returns all messages contained in the specified buffer
	Messages(string) []Message

	// Tries to receive a message and indicates if it was for them
	// and if any error occurred
	Receive(Message) (bool, error)

	// Returns the internal buffer struct they may contain
	Buffers() *Buffers

	// Returns the address corresponding to their endpoint
	Source() net.Addr

	// Returns the command asocciated data and whether
	// they are connected to the endpoint or not
	Online() (*cmds.Data, bool)

	// Returns the name of the server
	Name() string

	// Returns the context of the connection
	Connection() *Connection
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
func (t *TUI) addServer(name string, addr net.Addr, tls bool) error {
	if t.servers.Len() >= int(maxServers) {
		return ErrorMaxServers
	}

	_, ok := t.servers.Get(name)
	if ok {
		return ErrorExists
	}

	ip, err := net.ResolveTCPAddr("tcp4", addr.String())
	if err != nil {
		return err
	}

	if t.existsServer(*ip) {
		return ErrorExists
	}

	s := &RemoteServer{
		ip:   ip.IP,
		port: uint16(ip.Port),
		name: name,
		conn: &Connection{
			ctx:    context.Background(),
			cancel: func() {},
		},
		bufs: Buffers{
			tabs: models.NewTable[string, *tab](maxBuffers),
		},
		data: cmds.NewEmptyData(),
	}
	s.data.Waitlist = cmds.DefaultWaitlist()

	serv, err := db.SaveServer(
		t.data.DB,
		ip.IP.String(),
		uint16(ip.Port),
		name,
		tls,
	)
	if err != nil {
		return err
	}
	s.data.Server = &serv

	t.servers.Add(name, s)
	l := t.servers.Len()

	if tls {
		t.comp.servers.AddItem(name, addr.String()+" (TLS)", ascii(l), nil)
	} else {
		t.comp.servers.AddItem(name, addr.String(), ascii(l), nil)
	}

	t.renderServer(name)
	return nil
}

// Finds a server by a given address
func (t *TUI) existsServer(addr net.TCPAddr) bool {
	list := t.servers.GetAll()
	for _, v := range list {
		source := v.Source()
		if source == nil {
			continue
		}

		tcp, _ := net.ResolveTCPAddr("tcp4", source.String())
		if slices.Equal(tcp.IP, addr.IP) && tcp.Port == addr.Port {
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

	// Cleanup resources and wait a bit
	data, _ := s.Online()
	_ = cmds.Discn(
		s.Connection().Get(),
		cmds.Command{
			Output: t.systemMessage(),
			Data:   data,
			Static: &t.data,
		},
	)
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
	ip, _ := net.ResolveTCPAddr("tcp4", addr.String())
	db.RemoveServer(t.data.DB, ip.IP.String(), uint16(ip.Port))
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
		uname := data.User.User.Username
		t.comp.input.SetLabel(unameLabel(uname))
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

type RemoteServer struct {
	ip   net.IP
	port uint16
	name string

	conn *Connection

	bufs Buffers
	data *cmds.Data
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
	if msg.Source == nil {
		// Not this destination
		return false, nil
	}

	ip, err := net.ResolveTCPAddr("tcp4", msg.Source.String())
	if err != nil {
		// Not this destination
		return false, nil
	}

	cmp := slices.Compare(ip.IP, s.ip)
	if cmp != 0 || ip.Port != int(s.port) {
		// Not this destination
		return false, nil
	}

	check := strings.Replace(msg.Content, "\n", "", -1)
	if check == "" {
		// Empty content
		return false, ErrorNoText
	}

	b, ok := s.bufs.tabs.Get(msg.Buffer)
	if !ok {
		// s.bufs.New(msg.Buffer, false)
		// b, _ = s.bufs.tabs.Get(msg.Buffer)
		return false, nil
	}

	b.messages.Add(msg)
	return true, nil
}

func (s *RemoteServer) Buffers() *Buffers {
	return &s.bufs
}

func (s *RemoteServer) Online() (*cmds.Data, bool) {
	return s.data, s.data.IsConnected()
}

func (s *RemoteServer) Source() net.Addr {
	str := fmt.Sprintf("%s:%d", s.ip.String(), s.port)

	ip, err := net.ResolveTCPAddr("tcp4", str)
	if err != nil {
		panic("invalid IP in remote server")
	}

	return ip
}

func (s *RemoteServer) Name() string {
	return s.name
}

func (s *RemoteServer) Connection() *Connection {
	return s.conn
}

/* LOCAL SERVER */

type LocalServer struct {
	name string
	bufs Buffers
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
	if msg.Source != nil {
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

func (l *LocalServer) Connection() *Connection {
	return nil
}
