// Package match はローカルのGoプロセスだけで二人をマッチングし、
// サーバー権威で手番と盤面を確定して配信する対戦セッションを提供する。
//
// transportには依存しない。参加者への配信はchannelで行い、WebSocketや
// HTTPへの登録は#7が行う。
package match

import "github.com/ulxsth/nadesiko-reversi/server/internal/protocol"

// EventType は参加者へ配信する出来事の種別。
type EventType string

const (
	// EventWaiting は相手の参加を待っている。
	EventWaiting EventType = "waiting"
	// EventMatched は二人が揃ってroomが成立した。
	EventMatched EventType = "matched"
	// EventState は確定したgame state全体の配信。
	EventState EventType = "state"
	// EventOpponentLeft は相手が切断した。
	EventOpponentLeft EventType = "opponentLeft"
	// EventOpponentReturned は相手が再接続した。
	EventOpponentReturned EventType = "opponentReturned"
	// EventClosed はroomが終了した。
	EventClosed EventType = "closed"
)

// Event はroomの参加者へ配信する1通。
//
// 契約どおり差分ではなくstate全体を運ぶ。欠落検出の連番は持たず、
// 取りこぼしても次の配信で追いつく。
type Event struct {
	// Type は出来事の種別。
	Type EventType `json:"type"`
	// RoomID は対象のroom。
	RoomID string `json:"roomId"`
	// Seat は受信者の席。
	Seat protocol.Player `json:"seat"`
	// State は確定したgame state全体。waiting時はnil。
	State *protocol.State `json:"state,omitempty"`
	// GameEvent は受理したcommandの結果。state配信以外ではnil。
	GameEvent *protocol.Event `json:"event,omitempty"`
	// Reason は終了や切断の理由。表示可能な日本語。
	Reason string `json:"reason,omitempty"`
}

// clone はstateとgame eventを複製して、受信側の変更が共有状態へ伝播しないようにする。
func (e Event) clone() Event {
	out := e
	if e.State != nil {
		state := e.State.Clone()
		out.State = &state
	}
	if e.GameEvent != nil {
		gameEvent := *e.GameEvent
		if e.GameEvent.Changes != nil {
			gameEvent.Changes = append([]protocol.Change(nil), e.GameEvent.Changes...)
		}
		out.GameEvent = &gameEvent
	}
	return out
}
