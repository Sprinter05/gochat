package ui

import (
	"net"
	"slices"
	"strings"

	"github.com/Sprinter05/gochat/internal/models"
)

// SERVER INTERFACE

type Server interface {
	Messages(string) []Message
	Receive(Message) error
	Buffers() *Buffers
}

func (t *TUI) Active() Server {
	s, ok := t.servers.Get(t.active)
	if !ok {
		panic("active server does not exist")
	}

	return s
}

func (t *TUI) addServer(name string, addr net.Addr) {
	ip, err := net.ResolveTCPAddr("tcp4", addr.String())
	if err != nil {
		t.showError(err)
	}

	s := &RemoteServer{
		ip:   ip.IP,
		port: int16(ip.Port),
		name: name,
		bufs: Buffers{
			tabs: models.NewTable[string, *tab](maxBuffers),
		},
	}

	t.servers.Add(name, s)
}

// REMOTE SERVER

type RemoteServer struct {
	ip   net.IP
	port int16
	name string
	bufs Buffers
}

func (s *RemoteServer) Messages(name string) []Message {
	t, ok := s.bufs.tabs.Get(name)
	if !ok {
		return nil
	}

	return t.messages.Copy(0)
}

func (s *RemoteServer) Receive(msg Message) error {
	ip, err := net.ResolveTCPAddr("tcp4", msg.Source.String())
	if err != nil {
		// Not this destination
		return nil
	}

	cmp := slices.Compare(ip.IP, s.ip)
	if cmp != 0 || ip.Port != int(s.port) {
		// Not this destination
		return nil
	}

	check := strings.Replace(msg.Content, "\n", "", -1)
	if check == "" {
		// Empty content
		return nil
	}

	b, ok := s.bufs.tabs.Get(msg.Sender)
	if !ok {
		s.bufs.New(msg.Sender, false)
		b, _ = s.bufs.tabs.Get(msg.Sender)
	}

	b.messages.Add(msg)
	return nil
}

func (s *RemoteServer) Buffers() *Buffers {
	return &s.bufs
}

// LOCAL SERVER

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
		Content: Logo,
	}

	ret = append(ret, logo)
	ret = append(ret, msgs...)

	return ret
}

func (l *LocalServer) Receive(msg Message) error {
	b, ok := l.bufs.tabs.Get(msg.Sender)
	if !ok {
		// Not for this server
		return nil
	}

	if b.system && msg.Sender == selfSender {
		return ErrorSystemBuf
	}

	b.messages.Add(msg)
	return nil
}

func (l *LocalServer) Buffers() *Buffers {
	return &l.bufs
}
