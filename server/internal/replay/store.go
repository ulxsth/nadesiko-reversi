package replay

import (
	"context"
	"math/rand"
	"sync"
)

// Entry は保管庫が持つ1対局分。
// 契約では棋譜が保存形式の正本なので、Sourceをそのまま保持し、
// Recordは再生で確定したメタ情報として添える。
type Entry struct {
	Record Record
	Source string
}

// Clone はentryを複製する。
func (e Entry) Clone() Entry {
	return Entry{Record: e.Record.Clone(), Source: e.Source}
}

// Store は確定した対局の保管庫。
// P0ではプロセス内保存で足りるが、records/<gameId>.nako3への永続化実装へ
// 差し替えられるよう、保存の入口をこのinterfaceへ閉じる。
type Store interface {
	// Save は対局を保存する。同じgameIdは上書きする。
	Save(ctx context.Context, entry Entry) error
	// List は保存順に要約を返す。
	List(ctx context.Context) ([]Summary, error)
	// Get はgameIdで1件返す。見つからない場合は*NotFoundError。
	Get(ctx context.Context, gameID string) (*Entry, error)
	// Random は保存済みの中から1件返す。空の場合は*NotFoundError。
	Random(ctx context.Context) (*Entry, error)
}

// MemoryStore はプロセス内に保持するStore実装。
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]Entry
	order   []string

	// intn は乱数の取り出し口。testで固定できるようにする。
	intn func(n int) int
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore は空の保管庫を作る。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make(map[string]Entry),
		intn:    rand.Intn,
	}
}

// WithRandom は乱数の取り出し口を差し替える。testでRandomを決定的にする。
func (s *MemoryStore) WithRandom(intn func(n int) int) *MemoryStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.intn = intn
	return s
}

// Save は対局を保存する。保存順は最初に現れた順を保つ。
func (s *MemoryStore) Save(_ context.Context, entry Entry) error {
	if err := ValidateGameID(entry.Record.GameID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	gameID := entry.Record.GameID
	if _, exists := s.entries[gameID]; !exists {
		s.order = append(s.order, gameID)
	}
	s.entries[gameID] = entry.Clone()
	return nil
}

// List は保存順に要約を返す。
func (s *MemoryStore) List(_ context.Context) ([]Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Summary, 0, len(s.order))
	for _, gameID := range s.order {
		entry, ok := s.entries[gameID]
		if !ok {
			continue
		}
		out = append(out, entry.Record.Summary())
	}
	return out, nil
}

// Get はgameIdで1件返す。
func (s *MemoryStore) Get(_ context.Context, gameID string) (*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[gameID]
	if !ok {
		return nil, &NotFoundError{GameID: gameID}
	}
	clone := entry.Clone()
	return &clone, nil
}

// Random は保存済みの中から1件返す。
func (s *MemoryStore) Random(_ context.Context) (*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.order) == 0 {
		return nil, &NotFoundError{}
	}
	gameID := s.order[s.intn(len(s.order))]
	entry, ok := s.entries[gameID]
	if !ok {
		return nil, &NotFoundError{GameID: gameID}
	}
	clone := entry.Clone()
	return &clone, nil
}

// Len は保存件数を返す。
func (s *MemoryStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.order)
}
