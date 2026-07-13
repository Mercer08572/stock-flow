package auth

import (
	"context"
	"crypto/sha256"
	"sync"
)

type SessionStore interface {
	Save(ctx context.Context, session Session) error
	Get(ctx context.Context, token string) (Session, bool, error)
	Delete(ctx context.Context, token string) error
}

type inMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[[sha256.Size]byte]Session
}

func NewInMemorySessionStore() SessionStore {
	return &inMemorySessionStore{sessions: make(map[[sha256.Size]byte]Session)}
}

func (s *inMemorySessionStore) Save(_ context.Context, session Session) error {
	key := sha256.Sum256([]byte(session.Token))
	session.Token = ""

	s.mu.Lock()
	s.sessions[key] = session
	s.mu.Unlock()
	return nil
}

func (s *inMemorySessionStore) Get(_ context.Context, token string) (Session, bool, error) {
	key := sha256.Sum256([]byte(token))

	s.mu.RLock()
	session, ok := s.sessions[key]
	s.mu.RUnlock()
	return session, ok, nil
}

func (s *inMemorySessionStore) Delete(_ context.Context, token string) error {
	key := sha256.Sum256([]byte(token))

	s.mu.Lock()
	delete(s.sessions, key)
	s.mu.Unlock()
	return nil
}
