package ui

import (
	"fmt"
	"net"
	"slices"
	"strings"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
)

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

// Adds a server connected to a remote endpoint, stores it in
// the database, adds it to the TUI and changes to it.
func (t *TUI) addServer(name string, addr net.Addr) {
	if t.servers.Len() >= int(maxServers) {
		t.showError(ErrorMaxServers)
		return
	}

	_, ok := t.servers.Get(name)
	if ok {
		t.showError(ErrorExists)
		return
	}

	ip, err := net.ResolveTCPAddr("tcp4", addr.String())
	if err != nil {
		t.showError(err)
		return
	}

	s := &RemoteServer{
		ip:   ip.IP,
		port: uint16(ip.Port),
		name: name,
		bufs: Buffers{
			tabs: models.NewTable[string, *tab](maxBuffers),
		},
		data: new(cmds.Data),
	}

	s.data.Server = db.SaveServer(
		t.data.DB,
		ip.IP.String(),
		uint16(ip.Port),
		name,
	)

	t.servers.Add(name, s)
	l := t.servers.Len()
	t.comp.servers.AddItem(name, addr.String(), ascii(l), nil)

	t.renderServer(name)
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

// Deletes a server and all its contents (not in the database).
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

	t.servers.Remove(name)

	// addr := s.Source()
	// ip, _ := net.ResolveTCPAddr("tcp4", addr.String())
	// db.RemoveServer(t.data.DB, ip.IP.String(), uint16(ip.Port))

	t.renderServer(localServer)
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

	_, online := s.Online()
	if online {
		t.comp.servers.SetSelectedTextColor(tcell.ColorGreen)
	} else {
		t.comp.servers.SetSelectedTextColor(tcell.ColorPurple)
	}

	t.comp.buffers.Clear()
	if s.Buffers().tabs.Len() == 0 {
		t.comp.text.Clear()
		return
	}

	tabs := s.Buffers().tabs.GetAll()
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

	bufs Buffers
	data *cmds.Data
}

func (s *RemoteServer) Messages(name string) []Message {
	t, ok := s.bufs.tabs.Get(name)
	if !ok {
		return nil
	}

	return t.messages.Copy(0)
}

// Returns true if received
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
		s.bufs.New(msg.Buffer, false)
		b, _ = s.bufs.tabs.Get(msg.Buffer)
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

// Does not return an error if the server is not the destionation remote
func (l *LocalServer) Receive(msg Message) (bool, error) {
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
