package auth

import (
	"sync"
	"time"
)

type stateEntry struct {
	data      StateData
	expiresAt time.Time
}

type inMemStateStore struct {
	mu   sync.Mutex
	data map[string]stateEntry
}

// NewStateStore는 동시 사용에 안전한 인메모리 StateStore를 반환한다.
func NewStateStore() StateStore {
	return &inMemStateStore{data: make(map[string]stateEntry)}
}

func (s *inMemStateStore) Store(state string, d StateData, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[state] = stateEntry{data: d, expiresAt: time.Now().Add(ttl)}
}

// Consume은 state를 원자적으로 조회하고 삭제한다.
// 만료되었거나 존재하지 않으면 (zero, false)를 반환한다.
func (s *inMemStateStore) Consume(state string) (StateData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.data[state]
	if !ok {
		return StateData{}, false
	}
	delete(s.data, state)
	if time.Now().After(entry.expiresAt) {
		return StateData{}, false
	}
	return entry.data, true
}
