package session

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type Session struct {
	ID        string
	UserID    string
	Username  string
	Role      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
	stopCh   chan struct{}
}

func NewStore(ttl time.Duration) *Store {
	s := &Store{
		sessions: make(map[string]*Session),
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}
	go s.cleanup()
	return s
}

func (s *Store) Create(userID, username, role string) (*Session, error) {
	return s.CreateWithTTL(userID, username, role, s.ttl)
}

// CreateWithTTL creates a session with a custom TTL.
func (s *Store) CreateWithTTL(userID, username, role string, ttl time.Duration) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        id,
		UserID:    userID,
		Username:  username,
		Role:      role,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}

	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()

	return session, nil
}

func (s *Store) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok || time.Now().After(session.ExpiresAt) {
		return nil, false
	}
	return session, true
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// DeleteByUserID removes all sessions for a user.
func (s *Store) DeleteByUserID(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		if sess.UserID == userID {
			delete(s.sessions, id)
		}
	}
}

func (s *Store) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			for id, session := range s.sessions {
				if time.Now().After(session.ExpiresAt) {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

// Close stops the cleanup goroutine.
func (s *Store) Close() {
	close(s.stopCh)
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
