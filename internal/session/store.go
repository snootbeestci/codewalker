package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Store is an in-memory session registry.  v2 concern: persistence.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{sessions: make(map[string]*Session)}
}

// Set registers a session.
func (s *Store) Set(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
}

// Get retrieves a session by ID and resets its eviction clock.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	sess.Touch()
	return sess, nil
}

// Delete removes a session.
func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// All returns a snapshot of all active sessions.
func (s *Store) All() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, sess)
	}
	return out
}

// Len returns the number of active sessions.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// StartEviction launches a background goroutine that evicts sessions idle
// longer than ttl, checked every interval. It stops when ctx is cancelled.
func (s *Store) StartEviction(ctx context.Context, ttl time.Duration, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.evict(ttl)
			}
		}
	}()
}

func (s *Store) evict(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-ttl)
	for id, sess := range s.sessions {
		if sess.LastAccessed.Before(cutoff) {
			idle := time.Since(sess.LastAccessed)
			slog.Debug("session evicted", "session_id", id, "idle_duration", idle)
			delete(s.sessions, id)
		}
	}
}
