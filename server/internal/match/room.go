package match

import (
	"context"
	"sync"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

// eventBuffer は1席あたりの配信バッファ。
// 溢れた配信は捨てる。契約どおり欠落検出は行わず、次のstate配信で追いつく。
const eventBuffer = 16

// seat はroomの1席。切断してもplayerIDは残し、同じIDの再接続を受け入れる。
type seat struct {
	playerID  string
	events    chan Event
	connected bool
}

// Room は二人ぶんの席と、サーバー権威のgame stateを持つ1対局。
type Room struct {
	id     string
	runner runtime.Runner
	seed   uint32

	mu    sync.Mutex
	seats map[protocol.Player]*seat
	state *protocol.State
	phase RoomPhase
}

// newRoom は空席のroomを作る。
func newRoom(id string, runner runtime.Runner, seed uint32) *Room {
	return &Room{
		id:     id,
		runner: runner,
		seed:   seed,
		seats:  make(map[protocol.Player]*seat, 2),
		phase:  PhaseWaiting,
	}
}

// ID はroomの識別子を返す。
func (r *Room) ID() string { return r.id }

// Phase はroomの進行状態を返す。
func (r *Room) Phase() RoomPhase {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.phase
}

// Snapshot は現在の確定stateの複製を返す。未開始ならnil。
func (r *Room) Snapshot() *protocol.State {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		return nil
	}
	state := r.state.Clone()
	return &state
}

// Seats は席に着いているplayerIDを返す。
func (r *Room) Seats() map[protocol.Player]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make(map[protocol.Player]string, len(r.seats))
	for player, s := range r.seats {
		if s != nil {
			out[player] = s.playerID
		}
	}
	return out
}

// openSeat は空いている席を返す。dark、lightの順に埋める。
func (r *Room) openSeat() (protocol.Player, bool) {
	for _, player := range []protocol.Player{protocol.PlayerDark, protocol.PlayerLight} {
		if _, taken := r.seats[player]; !taken {
			return player, true
		}
	}
	return "", false
}

// seatOf はplayerIDが着いている席を返す。
func (r *Room) seatOf(playerID string) (protocol.Player, *seat, bool) {
	for player, s := range r.seats {
		if s != nil && s.playerID == playerID {
			return player, s, true
		}
	}
	return "", nil, false
}

// deliver は1席へ配信する。バッファが詰まっていたら捨てる。
func deliver(s *seat, event Event) {
	if s == nil || !s.connected || s.events == nil {
		return
	}
	select {
	case s.events <- event.clone():
	default:
		// 取りこぼしても次のstate配信で追いつくため、詰まったら捨てる
	}
}

// broadcast は接続中の全席へ配信する。席ごとにSeatを埋める。
func (r *Room) broadcast(build func(protocol.Player) Event) {
	for player, s := range r.seats {
		if s == nil || !s.connected {
			continue
		}
		event := build(player)
		event.RoomID = r.id
		event.Seat = player
		deliver(s, event)
	}
}

// stateEvent は確定state配信のeventを組み立てる。
func (r *Room) stateEvent(gameEvent *protocol.Event) func(protocol.Player) Event {
	return func(protocol.Player) Event {
		return Event{Type: EventState, State: r.state, GameEvent: gameEvent}
	}
}

// join は席に着く。同じplayerIDが切断中の席にいれば再接続として扱う。
// 戻り値は席、配信channel、再接続かどうか。
func (r *Room) join(ctx context.Context, playerID string) (protocol.Player, <-chan Event, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.phase == PhaseClosed {
		return "", nil, false, &JoinError{PlayerID: playerID, Message: "roomは終了しています"}
	}

	// 再接続: 席を保ったまま新しいchannelを渡す
	if player, s, found := r.seatOf(playerID); found {
		if s.connected {
			return "", nil, false, &JoinError{PlayerID: playerID, Message: "既に参加しています"}
		}
		s.events = make(chan Event, eventBuffer)
		s.connected = true
		if r.phase == PhaseSuspended && r.bothConnectedLocked() {
			r.phase = PhasePlaying
		}

		// 接続直後にも現在state全体を1通配信する
		deliver(s, Event{Type: EventState, RoomID: r.id, Seat: player, State: r.state})
		r.notifyOthersLocked(playerID, EventOpponentReturned, "相手が再接続しました")
		return player, s.events, true, nil
	}

	player, ok := r.openSeat()
	if !ok {
		return "", nil, false, &JoinError{PlayerID: playerID, Message: "roomは満席です"}
	}

	newSeat := &seat{playerID: playerID, events: make(chan Event, eventBuffer), connected: true}
	r.seats[player] = newSeat

	if len(r.seats) < 2 {
		r.phase = PhaseWaiting
		deliver(newSeat, Event{Type: EventWaiting, RoomID: r.id, Seat: player})
		return player, newSeat.events, false, nil
	}

	// 二人揃ったので対局を作り、両者へ成立と初期stateを配信する
	response, err := runtime.NewGame(ctx, r.runner, r.id, r.seed)
	if err != nil {
		delete(r.seats, player)
		return "", nil, false, err
	}
	if !response.OK || response.State == nil {
		delete(r.seats, player)
		message := "対局を開始できません"
		if response != nil && response.Error != nil {
			message = response.Error.Message
		}
		return "", nil, false, &JoinError{PlayerID: playerID, Message: message}
	}

	state := response.State.Clone()
	r.state = &state
	r.phase = PhasePlaying
	r.broadcast(func(protocol.Player) Event {
		return Event{Type: EventMatched, State: r.state}
	})

	return player, newSeat.events, false, nil
}

// bothConnectedLocked は二席とも接続中かを返す。呼び出し側でmuを保持する。
func (r *Room) bothConnectedLocked() bool {
	if len(r.seats) < 2 {
		return false
	}
	for _, s := range r.seats {
		if s == nil || !s.connected {
			return false
		}
	}
	return true
}

// notifyOthersLocked は指定playerID以外の席へ通知する。呼び出し側でmuを保持する。
func (r *Room) notifyOthersLocked(playerID string, eventType EventType, reason string) {
	for player, s := range r.seats {
		if s == nil || s.playerID == playerID || !s.connected {
			continue
		}
		deliver(s, Event{Type: eventType, RoomID: r.id, Seat: player, State: r.state, Reason: reason})
	}
}

// submit は1件のcommandを評価し、受理されたらroom全体へ配信する。
//
// 拒否はroomへ配信せず、送信者へ変更前stateを含むresponseを返す。
// 重複と競合はルール側のexpectedTurn判定に委ね、commandIdは記憶しない。
func (r *Room) submit(ctx context.Context, playerID string, command protocol.Command) (*protocol.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	player, _, found := r.seatOf(playerID)
	if !found {
		return rejected(protocol.CodeInvalidRequest, "このroomに参加していません", r.state), nil
	}
	if r.phase == PhaseClosed {
		return rejected(protocol.CodeGameFinished, "roomは終了しています", r.state), nil
	}
	if r.state == nil {
		return rejected(protocol.CodeInvalidState, "相手の参加を待っています", nil), nil
	}
	// 席と名乗るplayerが食い違うcommandは、相手の手番を騙れないよう拒否する
	if command.Player != player {
		return rejected(protocol.CodeNotYourTurn, "自分の席以外のcommandは送れません", r.state), nil
	}
	if protocolErr := command.Validate(); protocolErr != nil {
		return rejected(protocolErr.Code, protocolErr.Message, r.state), nil
	}

	authoritative := r.state.Clone()
	response, err := runtime.ApplyCommand(ctx, r.runner, authoritative, command)
	if err != nil {
		return nil, err
	}

	if !response.OK || response.State == nil {
		// 拒否は配信せず、送信者にだけ変更前stateを返す
		if response.State == nil {
			state := r.state.Clone()
			response.State = &state
		}
		return cloneResponse(response), nil
	}

	state := response.State.Clone()
	r.state = &state
	if state.Phase == protocol.PhaseFinished {
		r.phase = PhaseClosed
	}

	r.broadcast(r.stateEvent(response.Event))
	if r.phase == PhaseClosed {
		r.broadcast(func(protocol.Player) Event {
			return Event{Type: EventClosed, State: r.state, Reason: "対局が終了しました"}
		})
	}

	return cloneResponse(response), nil
}

// leave は席を切断状態にする。二席とも切断したらroomを閉じる。
// 戻り値はroomが空になったかどうか。
func (r *Room) leave(playerID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, s, found := r.seatOf(playerID)
	if !found {
		return false, &LeaveError{PlayerID: playerID, Message: "このroomに参加していません"}
	}

	if s.connected {
		s.connected = false
		if s.events != nil {
			close(s.events)
			s.events = nil
		}
	}

	anyConnected := false
	for _, other := range r.seats {
		if other != nil && other.connected {
			anyConnected = true
			break
		}
	}

	if !anyConnected {
		r.closeLocked("全員が切断しました")
		return true, nil
	}

	if r.phase == PhasePlaying {
		r.phase = PhaseSuspended
	}
	r.notifyOthersLocked(playerID, EventOpponentLeft, "相手が切断しました")
	return false, nil
}

// closeLocked はroomを終了し、残っているchannelを閉じる。
// 呼び出し側でmuを保持する。
func (r *Room) closeLocked(reason string) {
	if r.phase == PhaseClosed {
		r.releaseSeatsLocked()
		return
	}
	r.phase = PhaseClosed

	for player, s := range r.seats {
		if s == nil || !s.connected {
			continue
		}
		deliver(s, Event{Type: EventClosed, RoomID: r.id, Seat: player, State: r.state, Reason: reason})
	}
	r.releaseSeatsLocked()
}

// releaseSeatsLocked は全席のchannelを閉じてリークを防ぐ。
func (r *Room) releaseSeatsLocked() {
	for _, s := range r.seats {
		if s == nil {
			continue
		}
		if s.events != nil {
			close(s.events)
			s.events = nil
		}
		s.connected = false
	}
}

// close はroomを外から終了させる。
func (r *Room) close(reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeLocked(reason)
}
