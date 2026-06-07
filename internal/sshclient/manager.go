package sshclient

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type SessionInfo struct {
	ID        string
	UserID    uuid.UUID
	ServerID  uuid.UUID
	Client    *Client
	Shell     *Session
	CreatedAt time.Time
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionInfo
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*SessionInfo)}
}

func (m *Manager) Add(userID, serverID uuid.UUID, client *Client, shell *Session) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	m.sessions[id] = &SessionInfo{
		ID:        id,
		UserID:    userID,
		ServerID:  serverID,
		Client:    client,
		Shell:     shell,
		CreatedAt: time.Now(),
	}
	return id
}

func (m *Manager) Get(id string) (*SessionInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return errors.New("会话不存在")
	}
	if s.Shell != nil {
		s.Shell.Close()
	}
	if s.Client != nil {
		s.Client.Close()
	}
	delete(m.sessions, id)
	return nil
}

func (m *Manager) RemoveByServer(serverID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.ServerID == serverID {
			if s.Shell != nil {
				s.Shell.Close()
			}
			if s.Client != nil {
				s.Client.Close()
			}
			delete(m.sessions, id)
		}
	}
}

func (m *Manager) ListByUser(userID uuid.UUID) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := []string{}
	for id, s := range m.sessions {
		if s.UserID == userID {
			ids = append(ids, id)
		}
	}
	return ids
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.Shell != nil {
			s.Shell.Close()
		}
		if s.Client != nil {
			s.Client.Close()
		}
		delete(m.sessions, id)
	}
}
