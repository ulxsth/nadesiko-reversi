package match

import (
	"fmt"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

// RoomPhase はroomの進行状態。
type RoomPhase string

const (
	// PhaseWaiting は席が埋まっていない。
	PhaseWaiting RoomPhase = "waiting"
	// PhasePlaying は二人が揃って対局中。
	PhasePlaying RoomPhase = "playing"
	// PhaseSuspended は切断により再接続待ち。
	PhaseSuspended RoomPhase = "suspended"
	// PhaseClosed はroomが終了した。
	PhaseClosed RoomPhase = "closed"
)

// JoinError は参加できない理由を表す。
type JoinError struct {
	PlayerID string
	Message  string
}

func (e *JoinError) Error() string {
	return fmt.Sprintf("参加できません (%s): %s", e.PlayerID, e.Message)
}

// LeaveError は退出を扱えない理由を表す。
type LeaveError struct {
	PlayerID string
	Message  string
}

func (e *LeaveError) Error() string {
	return fmt.Sprintf("退出を扱えません (%s): %s", e.PlayerID, e.Message)
}

// rejected は契約のerror codeを持つ拒否responseを組み立てる。
// roomの都合による拒否も、ルールによる拒否と同じ形でclientへ返す。
func rejected(code protocol.ErrorCode, message string, state *protocol.State) *protocol.Response {
	response := &protocol.Response{
		Version: protocol.Version,
		OK:      false,
		Error:   protocol.NewError(code, message),
	}
	if state != nil {
		clone := state.Clone()
		response.State = &clone
	}
	return response
}

// cloneResponse はresponseを複製して、呼び出し側の変更が共有状態へ伝播しないようにする。
func cloneResponse(response *protocol.Response) *protocol.Response {
	if response == nil {
		return nil
	}
	clone := *response
	if response.State != nil {
		state := response.State.Clone()
		clone.State = &state
	}
	if response.Event != nil {
		gameEvent := *response.Event
		if response.Event.Changes != nil {
			gameEvent.Changes = append([]protocol.Change(nil), response.Event.Changes...)
		}
		clone.Event = &gameEvent
	}
	if response.Error != nil {
		responseError := *response.Error
		clone.Error = &responseError
	}
	return &clone
}
