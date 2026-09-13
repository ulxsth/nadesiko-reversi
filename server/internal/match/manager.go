package match

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

// Membership は参加の結果。#7はこれを受け取ってtransportへ繋ぐ。
type Membership struct {
	// RoomID は割り当てられたroom。
	RoomID string
	// PlayerID は参加者の識別子。
	PlayerID string
	// Seat は割り当てられた席。
	Seat protocol.Player
	// Events は確定stateとroomの出来事を受け取るchannel。
	// roomが終了すると閉じる。
	Events <-chan Event
	// Reconnected は切断からの復帰かどうか。
	Reconnected bool
}

// Manager はroomの割り当てと寿命を管理する。
//
// 先着二人を1つのroomへ入れ、二人揃った時点で対局を作る。
// 全員が切断したroomは破棄して、roomとchannelが残らないようにする。
type Manager struct {
	runner runtime.Runner

	// 参加先の選択から席とplayerIDの登録までを、Join同士で直列化する。
	joinMu  sync.Mutex
	mu      sync.Mutex
	rooms   map[string]*Room
	players map[string]string
	waiting string
	nextID  int

	seedFor func(roomID string) uint32
}

// Option はManagerの構成を差し替える。
type Option func(*Manager)

// WithSeed は全roomで同じseedを使う。testで対局を決定的にする。
func WithSeed(seed uint32) Option {
	return func(m *Manager) {
		m.seedFor = func(string) uint32 { return seed }
	}
}

// WithSeedFunc はroomごとのseedの決め方を差し替える。
func WithSeedFunc(seedFor func(roomID string) uint32) Option {
	return func(m *Manager) {
		if seedFor != nil {
			m.seedFor = seedFor
		}
	}
}

// NewManager はmanagerを作る。
func NewManager(runner runtime.Runner, options ...Option) (*Manager, error) {
	if runner == nil {
		return nil, fmt.Errorf("runnerが必要です")
	}
	manager := &Manager{
		runner:  runner,
		rooms:   make(map[string]*Room),
		players: make(map[string]string),
		seedFor: func(string) uint32 { return rand.Uint32() },
	}
	for _, option := range options {
		option(manager)
	}
	return manager, nil
}

// Join は先着順にroomへ割り当てる。
//
// 空席のあるroomがあればそこへ入れ、無ければ新しいroomを作る。
// 同じplayerIDが切断中の席にいる場合は再接続として扱い、現在stateを1通配信する。
func (m *Manager) Join(ctx context.Context, playerID string) (*Membership, error) {
	if playerID == "" {
		return nil, &JoinError{Message: "playerIDが必要です"}
	}
	m.joinMu.Lock()
	defer m.joinMu.Unlock()

	room, err := m.roomForJoin(playerID)
	if err != nil {
		return nil, err
	}

	seat, events, reconnected, err := room.join(ctx, playerID)
	if err != nil {
		m.releaseOnJoinFailure(room)
		return nil, err
	}

	m.mu.Lock()
	m.players[playerID] = room.ID()
	if len(room.Seats()) >= 2 && m.waiting == room.ID() {
		m.waiting = ""
	}
	m.mu.Unlock()

	return &Membership{
		RoomID:      room.ID(),
		PlayerID:    playerID,
		Seat:        seat,
		Events:      events,
		Reconnected: reconnected,
	}, nil
}

// roomForJoin は参加先のroomを決める。
func (m *Manager) roomForJoin(playerID string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 既にroomにいるなら、そのroomで再接続を試す
	if roomID, ok := m.players[playerID]; ok {
		if room, exists := m.rooms[roomID]; exists {
			return room, nil
		}
		delete(m.players, playerID)
	}

	if m.waiting != "" {
		if room, exists := m.rooms[m.waiting]; exists && room.Phase() != PhaseClosed {
			return room, nil
		}
		m.waiting = ""
	}

	m.nextID++
	roomID := fmt.Sprintf("room-%d", m.nextID)
	room := newRoom(roomID, m.runner, m.seedFor(roomID))
	m.rooms[roomID] = room
	m.waiting = roomID
	return room, nil
}

// releaseOnJoinFailure は参加に失敗したroomが空なら片付ける。
func (m *Manager) releaseOnJoinFailure(room *Room) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(room.Seats()) > 0 {
		return
	}
	delete(m.rooms, room.ID())
	if m.waiting == room.ID() {
		m.waiting = ""
	}
}

// Submit は1件のcommandをroomへ渡し、受理されたら全席へ配信する。
//
// roomIDは呼び出し側が名乗ったroom。参加しているroomと違う場合は拒否する。
func (m *Manager) Submit(ctx context.Context, roomID, playerID string, command protocol.Command) (*protocol.Response, error) {
	m.mu.Lock()
	joined, ok := m.players[playerID]
	room := m.rooms[roomID]
	m.mu.Unlock()

	if !ok {
		return rejected(protocol.CodeInvalidRequest, "roomに参加していません", nil), nil
	}
	if joined != roomID {
		return rejected(protocol.CodeInvalidRequest, "参加していないroomへのcommandです", nil), nil
	}
	if room == nil {
		return rejected(protocol.CodeInvalidRequest, "roomが存在しません", nil), nil
	}

	return room.submit(ctx, playerID, command)
}

// Leave は切断を扱う。相手が残っていればroomを再接続待ちにし、
// 全員が切断したらroomを破棄する。
func (m *Manager) Leave(_ context.Context, playerID string) error {
	m.mu.Lock()
	roomID, ok := m.players[playerID]
	room := m.rooms[roomID]
	m.mu.Unlock()

	if !ok || room == nil {
		return &LeaveError{PlayerID: playerID, Message: "roomに参加していません"}
	}

	empty, err := room.leave(playerID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !empty {
		// 席はroomに残るので、同じplayerIDが再接続できるよう対応を保つ
		return nil
	}

	for _, seated := range room.Seats() {
		delete(m.players, seated)
	}
	delete(m.players, playerID)
	delete(m.rooms, roomID)
	if m.waiting == roomID {
		m.waiting = ""
	}
	return nil
}

// Room はroomIDに対応するroomを返す。
func (m *Manager) Room(roomID string) (*Room, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, ok := m.rooms[roomID]
	return room, ok
}

// RoomCount は保持しているroom数を返す。リークしていないことの確認に使う。
func (m *Manager) RoomCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rooms)
}

// PlayerCount は席に着いている参加者数を返す。
func (m *Manager) PlayerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.players)
}

// Close は全roomを終了して片付ける。
func (m *Manager) Close(reason string) {
	m.mu.Lock()
	rooms := make([]*Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	m.rooms = make(map[string]*Room)
	m.players = make(map[string]string)
	m.waiting = ""
	m.mu.Unlock()

	for _, room := range rooms {
		room.close(reason)
	}
}
