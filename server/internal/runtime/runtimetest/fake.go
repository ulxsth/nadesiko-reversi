// Package runtimetest はruntime.Runnerの差し替え実装を提供する。
// handlerやmatch serviceは実際のgonakoを起動せずにルール評価を差し替えられる。
package runtimetest

import (
	"context"
	"sync"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

// Fake は記録と応答差し替えができるRunner実装。複数goroutineから同時に使える。
type Fake struct {
	mu       sync.Mutex
	requests []protocol.Request

	// EvaluateFunc が設定されていればそれを呼ぶ。未設定なら固定の成功応答を返す。
	EvaluateFunc func(ctx context.Context, request protocol.Request) (*protocol.Response, error)
}

// Fakeがinterfaceを満たすことをcompile時に確認する。
var _ runtime.Runner = (*Fake)(nil)

// New は応答関数を持つFakeを作る。nilを渡すと既定の成功応答を返す。
func New(evaluate func(ctx context.Context, request protocol.Request) (*protocol.Response, error)) *Fake {
	return &Fake{EvaluateFunc: evaluate}
}

// Evaluate はrequestを記録し、設定された関数の結果を返す。
func (f *Fake) Evaluate(ctx context.Context, request protocol.Request) (*protocol.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, request)
	evaluate := f.EvaluateFunc
	f.mu.Unlock()

	if evaluate != nil {
		return evaluate(ctx, request)
	}
	return Accepted(NewState(request.GameID)), nil
}

// Requests は受け取ったrequestを受信順に返す。
func (f *Fake) Requests() []protocol.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]protocol.Request(nil), f.requests...)
}

// Len は受け取ったrequestの件数を返す。
func (f *Fake) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

// Reset は記録を消す。
func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = nil
}

// Accepted は成功responseを組み立てる。
func Accepted(state protocol.State) *protocol.Response {
	return &protocol.Response{Version: protocol.Version, OK: true, State: &state}
}

// Rejected は拒否responseを組み立てる。
func Rejected(code protocol.ErrorCode, message string, state protocol.State) *protocol.Response {
	return &protocol.Response{
		Version: protocol.Version,
		OK:      false,
		Error:   protocol.NewError(code, message),
		State:   &state,
	}
}

// NewState はseed=1の新規ゲームと同じ値を持つstateを返す。
// gonakoを起動できない環境でも、契約に沿ったstateでhandlerを試せる。
func NewState(gameID string) protocol.State {
	if gameID == "" {
		gameID = "fake-game"
	}

	board := protocol.NewBoard()
	board[protocol.Index(3, 3)] = protocol.NewCell(0)
	board[protocol.Index(4, 4)] = protocol.NewCell(0)
	board[protocol.Index(3, 4)] = protocol.NewCell(255)
	board[protocol.Index(4, 3)] = protocol.NewCell(255)

	return protocol.State{
		Version:           protocol.Version,
		GameID:            gameID,
		Board:             board,
		TurnNumber:        0,
		CurrentPlayer:     protocol.PlayerDark,
		NextColor:         60,
		RNGState:          1015568748,
		ConsecutivePasses: 0,
		Phase:             protocol.PhasePlaying,
		LegalMoves: []protocol.Position{
			{Row: 2, Col: 2}, {Row: 2, Col: 3}, {Row: 2, Col: 4}, {Row: 2, Col: 5},
			{Row: 3, Col: 2}, {Row: 3, Col: 5}, {Row: 4, Col: 2}, {Row: 4, Col: 5},
			{Row: 5, Col: 2}, {Row: 5, Col: 3}, {Row: 5, Col: 4}, {Row: 5, Col: 5},
		},
	}
}
