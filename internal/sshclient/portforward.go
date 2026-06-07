package sshclient

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/google/uuid"
)

type ForwardType string

const (
	ForwardTypeLocal  ForwardType = "local"
	ForwardTypeRemote ForwardType = "remote"
)

type ForwardInfo struct {
	ID         string
	UserID     uuid.UUID
	ServerID   uuid.UUID
	Type       ForwardType
	LocalHost  string
	LocalPort  int
	RemoteHost string
	RemotePort int
	listener   net.Listener
	conns      map[net.Conn]struct{}
	mu         sync.Mutex
	closed     bool
}

type PortForwardManager struct {
	mu       sync.RWMutex
	forwards map[string]*ForwardInfo
}

func NewPortForwardManager() *PortForwardManager {
	return &PortForwardManager{forwards: make(map[string]*ForwardInfo)}
}

func (m *PortForwardManager) StartLocal(f *ForwardInfo) error {
	if f.Type != ForwardTypeLocal {
		return fmt.Errorf("类型不匹配")
	}
	addr := fmt.Sprintf("%s:%d", f.LocalHost, f.LocalPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", addr, err)
	}
	f.listener = listener
	f.conns = make(map[net.Conn]struct{})
	f.ID = uuid.New().String()

	m.mu.Lock()
	m.forwards[f.ID] = f
	m.mu.Unlock()

	go m.acceptLocal(f)
	return nil
}

func (m *PortForwardManager) acceptLocal(f *ForwardInfo) {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go m.handleLocal(f, conn)
	}
}

func (m *PortForwardManager) handleLocal(f *ForwardInfo, localConn net.Conn) {
	f.mu.Lock()
	f.conns[localConn] = struct{}{}
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.conns, localConn)
		f.mu.Unlock()
		localConn.Close()
	}()

	sshClient := m.findClient(f.ServerID)
	if sshClient == nil {
		return
	}

	remoteConn, err := sshClient.DialTCP(f.RemoteHost, f.RemotePort)
	if err != nil {
		return
	}
	defer remoteConn.Close()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(remoteConn, localConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(localConn, remoteConn)
		done <- struct{}{}
	}()
	<-done
}

func (m *PortForwardManager) findClient(serverID uuid.UUID) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, f := range m.forwards {
		if f.ServerID == serverID {
			return nil
		}
	}
	return nil
}

func (m *PortForwardManager) StartRemote(client *Client, f *ForwardInfo) error {
	if f.Type != ForwardTypeRemote {
		return fmt.Errorf("类型不匹配")
	}
	listener, err := client.ListenTCP(f.RemoteHost, f.RemotePort)
	if err != nil {
		return fmt.Errorf("远程监听失败: %w", err)
	}
	f.listener = listener
	f.ID = uuid.New().String()

	m.mu.Lock()
	m.forwards[f.ID] = f
	m.mu.Unlock()

	go m.acceptRemote(f, client)
	return nil
}

func (m *PortForwardManager) acceptRemote(f *ForwardInfo, client *Client) {
	for {
		remoteConn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go m.handleRemote(f, client, remoteConn)
	}
}

func (m *PortForwardManager) handleRemote(f *ForwardInfo, client *Client, remoteConn net.Conn) {
	defer remoteConn.Close()

	localConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", f.LocalHost, f.LocalPort))
	if err != nil {
		return
	}
	defer localConn.Close()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(remoteConn, localConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(localConn, remoteConn)
		done <- struct{}{}
	}()
	<-done
}

func (m *PortForwardManager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.forwards[id]
	if !ok {
		return fmt.Errorf("转发不存在")
	}
	if f.listener != nil {
		f.listener.Close()
	}
	f.mu.Lock()
	for c := range f.conns {
		c.Close()
	}
	f.mu.Unlock()
	f.closed = true
	delete(m.forwards, id)
	return nil
}

func (m *PortForwardManager) StopByServer(serverID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, f := range m.forwards {
		if f.ServerID == serverID {
			if f.listener != nil {
				f.listener.Close()
			}
			delete(m.forwards, id)
		}
	}
}

func (m *PortForwardManager) List(userID uuid.UUID) []*ForwardInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []*ForwardInfo{}
	for _, f := range m.forwards {
		if f.UserID == userID {
			out = append(out, f)
		}
	}
	return out
}

func (m *PortForwardManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, f := range m.forwards {
		if f.listener != nil {
			f.listener.Close()
		}
		delete(m.forwards, id)
	}
}
