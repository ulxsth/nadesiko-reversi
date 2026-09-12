package match_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ulxsth/nadesiko-reversi/server/internal/match"
	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime/runtimetest"
)

// advancingRunner は手番だけを進めるstub runner。
//
// ルールは評価せず、expectedTurnが現在のturnNumberと合うかだけを見る。
// これによりmatch側が重複排除の状態を持たないことを確認できる。
func advancingRunner() *runtimetest.Fake {
	var mu sync.Mutex
	states := make(map[string]protocol.State)

	return runtimetest.New(func(_ context.Context, request protocol.Request) (*protocol.Response, error) {
		mu.Lock()
		defer mu.Unlock()

		switch request.Action {
		case protocol.ActionNewGame:
			state := runtimetest.NewState(request.GameID)
			states[request.GameID] = state
			return runtimetest.Accepted(state), nil

		case protocol.ActionApplyCommand:
			gameID := request.State.GameID
			current, ok := states[gameID]
			if !ok {
				current = *request.State
			}
			if request.Command.ExpectedTurn != current.TurnNumber {
				return runtimetest.Rejected(protocol.CodeStaleTurn, "turnが違います", current), nil
			}
			current.TurnNumber++
			current.CurrentPlayer = current.CurrentPlayer.Opponent()
			current.NextColor = uint8((int(current.NextColor) + 37) % 256)
			states[gameID] = current
			return runtimetest.Accepted(current), nil

		default:
			return runtimetest.Rejected(protocol.CodeInvalidRequest, "未対応のactionです", protocol.State{}), nil
		}
	})
}

// newManager はstub runnerを使うmanagerを作る。
func newManager(t *testing.T) *match.Manager {
	t.Helper()
	manager, err := match.NewManager(advancingRunner(), match.WithSeed(1))
	if err != nil {
		t.Fatalf("managerを作れません: %v", err)
	}
	t.Cleanup(func() { manager.Close("test終了") })
	return manager
}

// nextEvent は1通受け取る。来なければ失敗させる。
func nextEvent(t *testing.T, events <-chan match.Event) match.Event {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("channelが閉じています")
		}
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("eventが届きません")
		return match.Event{}
	}
}

// expectEvent は指定種別のeventが来るまで読み飛ばす。
func expectEvent(t *testing.T, events <-chan match.Event, want match.EventType) match.Event {
	t.Helper()
	for i := 0; i < 8; i++ {
		event := nextEvent(t, events)
		if event.Type == want {
			return event
		}
	}
	t.Fatalf("%sが届きません", want)
	return match.Event{}
}

// drain は溜まっているeventを読み捨てる。
func drain(events <-chan match.Event) {
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

func TestJoinPairsFirstTwoPlayers(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, err := manager.Join(ctx, "alice")
	if err != nil {
		t.Fatalf("1人目の参加に失敗: %v", err)
	}
	if first.Seat != protocol.PlayerDark {
		t.Errorf("1人目の席が違います: %q", first.Seat)
	}

	waiting := nextEvent(t, first.Events)
	if waiting.Type != match.EventWaiting {
		t.Fatalf("待機eventが届きません: %+v", waiting)
	}
	if waiting.RoomID != first.RoomID || waiting.Seat != protocol.PlayerDark {
		t.Errorf("待機eventの中身が違います: %+v", waiting)
	}

	second, err := manager.Join(ctx, "bob")
	if err != nil {
		t.Fatalf("2人目の参加に失敗: %v", err)
	}
	if second.RoomID != first.RoomID {
		t.Errorf("別のroomへ入りました: %q vs %q", second.RoomID, first.RoomID)
	}
	if second.Seat != protocol.PlayerLight {
		t.Errorf("2人目の席が違います: %q", second.Seat)
	}

	// 両者へ成立と初期stateが届く
	for name, events := range map[string]<-chan match.Event{"alice": first.Events, "bob": second.Events} {
		matched := expectEvent(t, events, match.EventMatched)
		if matched.State == nil {
			t.Errorf("%s: 成立eventにstateがありません", name)
			continue
		}
		if matched.State.TurnNumber != 0 || matched.State.CurrentPlayer != protocol.PlayerDark {
			t.Errorf("%s: 初期stateが違います: %+v", name, matched.State)
		}
	}

	if manager.RoomCount() != 1 {
		t.Errorf("room数が違います: %d", manager.RoomCount())
	}
}

func TestThirdPlayerOpensNewRoom(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	third, err := manager.Join(ctx, "carol")
	if err != nil {
		t.Fatalf("3人目の参加に失敗: %v", err)
	}

	if third.RoomID == first.RoomID {
		t.Error("満席のroomへ3人目が入りました")
	}
	if third.Seat != protocol.PlayerDark {
		t.Errorf("3人目の席が違います: %q", third.Seat)
	}
	if manager.RoomCount() != 2 {
		t.Errorf("room数が違います: %d", manager.RoomCount())
	}
	drain(first.Events)
	drain(second.Events)

	waiting := nextEvent(t, third.Events)
	if waiting.Type != match.EventWaiting {
		t.Errorf("3人目に待機eventが届きません: %+v", waiting)
	}
}

func TestSubmitBroadcastsFullStateToBothSeats(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	expectEvent(t, first.Events, match.EventMatched)
	expectEvent(t, second.Events, match.EventMatched)

	response, err := manager.Submit(ctx, first.RoomID, "alice",
		protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2))
	if err != nil {
		t.Fatalf("着手に失敗: %v", err)
	}
	if !response.OK {
		t.Fatalf("着手が拒否されました: %+v", response.Error)
	}

	// 差分ではなくstate全体が両者へ届く
	for name, events := range map[string]<-chan match.Event{"alice": first.Events, "bob": second.Events} {
		event := expectEvent(t, events, match.EventState)
		if event.State == nil {
			t.Errorf("%s: state配信にstateがありません", name)
			continue
		}
		if len(event.State.Board) != protocol.BoardCells {
			t.Errorf("%s: 盤面全体が届いていません: %d", name, len(event.State.Board))
		}
		if event.State.TurnNumber != 1 {
			t.Errorf("%s: turnNumberが違います: %d", name, event.State.TurnNumber)
		}
		if event.State.CurrentPlayer != protocol.PlayerLight {
			t.Errorf("%s: 次手番が違います: %q", name, event.State.CurrentPlayer)
		}
		if event.State.Phase != protocol.PhasePlaying {
			t.Errorf("%s: phaseが届いていません: %q", name, event.State.Phase)
		}
	}
}

func TestDuplicateExpectedTurnIsRejectedAsStale(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	expectEvent(t, first.Events, match.EventMatched)
	expectEvent(t, second.Events, match.EventMatched)

	command := protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2)

	accepted, err := manager.Submit(ctx, first.RoomID, "alice", command)
	if err != nil {
		t.Fatalf("1件目に失敗: %v", err)
	}
	if !accepted.OK {
		t.Fatalf("1件目が拒否されました: %+v", accepted.Error)
	}
	expectEvent(t, first.Events, match.EventState)
	expectEvent(t, second.Events, match.EventState)

	// 同じcommandの再送。commandIdも同じだが、判定はexpectedTurnだけで行う。
	stale, err := manager.Submit(ctx, first.RoomID, "alice", command)
	if err != nil {
		t.Fatalf("2件目に失敗: %v", err)
	}
	if stale.OK {
		t.Fatal("同じexpectedTurnの2件目が受理されました")
	}
	if stale.Error.Code != protocol.CodeStaleTurn {
		t.Errorf("codeが違います: %q", stale.Error.Code)
	}
	if stale.State == nil {
		t.Fatal("拒否応答に現在stateがありません")
	}
	if stale.State.TurnNumber != 1 {
		t.Errorf("拒否で盤面が進みました: %d", stale.State.TurnNumber)
	}

	// 拒否はroomへ配信しない
	select {
	case event := <-second.Events:
		t.Errorf("拒否が相手へ配信されました: %+v", event)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDifferentCommandIdWithSameTurnIsStillStale(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	expectEvent(t, first.Events, match.EventMatched)
	expectEvent(t, second.Events, match.EventMatched)

	if _, err := manager.Submit(ctx, first.RoomID, "alice",
		protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2)); err != nil {
		t.Fatalf("1件目に失敗: %v", err)
	}
	drain(first.Events)
	drain(second.Events)

	// commandIdを変えても、expectedTurnが古ければstale_turnになる。
	// match側がcommandIdで重複排除していないことの確認。
	stale, err := manager.Submit(ctx, first.RoomID, "alice",
		protocol.PlaceCommand("cmd-2", protocol.PlayerDark, 0, 3, 2))
	if err != nil {
		t.Fatalf("2件目に失敗: %v", err)
	}
	if stale.OK || stale.Error.Code != protocol.CodeStaleTurn {
		t.Errorf("stale_turnになりません: %+v", stale)
	}
}

func TestSubmitRejectsWrongRoomAndNonMember(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	third, _ := manager.Join(ctx, "carol")
	expectEvent(t, first.Events, match.EventMatched)
	drain(second.Events)
	drain(third.Events)

	command := protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2)

	// 別roomのIDを名乗る
	wrongRoom, err := manager.Submit(ctx, third.RoomID, "alice", command)
	if err != nil {
		t.Fatalf("評価に失敗: %v", err)
	}
	if wrongRoom.OK || wrongRoom.Error.Code != protocol.CodeInvalidRequest {
		t.Errorf("別roomのcommandが拒否されません: %+v", wrongRoom)
	}

	// 参加していないplayer
	stranger, err := manager.Submit(ctx, first.RoomID, "dave", command)
	if err != nil {
		t.Fatalf("評価に失敗: %v", err)
	}
	if stranger.OK || stranger.Error.Code != protocol.CodeInvalidRequest {
		t.Errorf("非参加者のcommandが拒否されません: %+v", stranger)
	}
}

func TestSubmitRejectsSeatMismatch(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	expectEvent(t, first.Events, match.EventMatched)
	drain(second.Events)

	// aliceはdark席。lightを名乗るcommandは拒否する。
	response, err := manager.Submit(ctx, first.RoomID, "alice",
		protocol.PlaceCommand("cmd-1", protocol.PlayerLight, 0, 2, 2))
	if err != nil {
		t.Fatalf("評価に失敗: %v", err)
	}
	if response.OK || response.Error.Code != protocol.CodeNotYourTurn {
		t.Errorf("席違いのcommandが拒否されません: %+v", response)
	}
}

func TestSubmitBeforeMatchIsRejected(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	nextEvent(t, first.Events)

	response, err := manager.Submit(ctx, first.RoomID, "alice",
		protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2))
	if err != nil {
		t.Fatalf("評価に失敗: %v", err)
	}
	if response.OK || response.Error.Code != protocol.CodeInvalidState {
		t.Errorf("相手待ちのcommandが拒否されません: %+v", response)
	}
}

func TestLeaveSuspendsRoomAndNotifiesOpponent(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	expectEvent(t, first.Events, match.EventMatched)
	expectEvent(t, second.Events, match.EventMatched)

	if err := manager.Leave(ctx, "alice"); err != nil {
		t.Fatalf("退出に失敗: %v", err)
	}

	left := expectEvent(t, second.Events, match.EventOpponentLeft)
	if left.Reason == "" {
		t.Error("切断の理由が空です")
	}

	room, ok := manager.Room(first.RoomID)
	if !ok {
		t.Fatal("roomが消えました")
	}
	if room.Phase() != match.PhaseSuspended {
		t.Errorf("再接続待ちになりません: %q", room.Phase())
	}

	// 切断した側のchannelは閉じる
	if _, open := <-first.Events; open {
		t.Error("切断した席のchannelが閉じていません")
	}
}

func TestReconnectRestoresSeatAndSendsCurrentState(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	expectEvent(t, first.Events, match.EventMatched)
	expectEvent(t, second.Events, match.EventMatched)

	if _, err := manager.Submit(ctx, first.RoomID, "alice",
		protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2)); err != nil {
		t.Fatalf("着手に失敗: %v", err)
	}
	drain(first.Events)
	drain(second.Events)

	if err := manager.Leave(ctx, "alice"); err != nil {
		t.Fatalf("退出に失敗: %v", err)
	}
	drain(second.Events)

	back, err := manager.Join(ctx, "alice")
	if err != nil {
		t.Fatalf("再接続に失敗: %v", err)
	}
	if !back.Reconnected {
		t.Error("再接続として扱われていません")
	}
	if back.RoomID != first.RoomID || back.Seat != protocol.PlayerDark {
		t.Errorf("席が変わりました: %+v", back)
	}

	// 再接続直後にも現在state全体が1通届く
	state := expectEvent(t, back.Events, match.EventState)
	if state.State == nil || state.State.TurnNumber != 1 {
		t.Errorf("現在stateが届きません: %+v", state.State)
	}

	returned := expectEvent(t, second.Events, match.EventOpponentReturned)
	if returned.Reason == "" {
		t.Error("復帰の理由が空です")
	}

	room, _ := manager.Room(first.RoomID)
	if room.Phase() != match.PhasePlaying {
		t.Errorf("対局中へ戻りません: %q", room.Phase())
	}
}

func TestRoomIsReleasedWhenEveryoneLeaves(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	expectEvent(t, first.Events, match.EventMatched)
	expectEvent(t, second.Events, match.EventMatched)

	if err := manager.Leave(ctx, "alice"); err != nil {
		t.Fatalf("退出に失敗: %v", err)
	}
	if manager.RoomCount() != 1 {
		t.Errorf("片方の退出でroomが消えました: %d", manager.RoomCount())
	}

	if err := manager.Leave(ctx, "bob"); err != nil {
		t.Fatalf("退出に失敗: %v", err)
	}
	if manager.RoomCount() != 0 {
		t.Errorf("roomが残っています: %d", manager.RoomCount())
	}
	if manager.PlayerCount() != 0 {
		t.Errorf("参加者が残っています: %d", manager.PlayerCount())
	}

	// 残った側のchannelも閉じる
	for range second.Events {
	}
}

func TestLeaveUnknownPlayer(t *testing.T) {
	manager := newManager(t)
	if err := manager.Leave(context.Background(), "nobody"); err == nil {
		t.Fatal("未参加のplayerの退出がエラーになりません")
	}
}

func TestConcurrentSubmitsAcceptOnlyOnePerTurn(t *testing.T) {
	ctx := context.Background()
	manager := newManager(t)

	first, _ := manager.Join(ctx, "alice")
	second, _ := manager.Join(ctx, "bob")
	expectEvent(t, first.Events, match.EventMatched)
	expectEvent(t, second.Events, match.EventMatched)

	// 配信channelが詰まらないよう読み続ける
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-first.Events:
			case <-second.Events:
			case <-time.After(500 * time.Millisecond):
				return
			}
		}
	}()

	const workers = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			response, err := manager.Submit(ctx, first.RoomID, "alice",
				protocol.PlaceCommand("cmd-race", protocol.PlayerDark, 0, 2, 2))
			if err != nil {
				t.Errorf("評価に失敗: %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if response.OK {
				accepted++
			} else if response.Error.Code != protocol.CodeStaleTurn {
				t.Errorf("想定外の拒否: %q", response.Error.Code)
			}
		}(i)
	}
	wg.Wait()
	<-done

	if accepted != 1 {
		t.Errorf("同じturnで%d件受理されました", accepted)
	}
}

func TestManagerRequiresRunner(t *testing.T) {
	if _, err := match.NewManager(nil); err == nil {
		t.Fatal("runnerなしでmanagerが作れました")
	}
}

func TestJoinRequiresPlayerID(t *testing.T) {
	manager := newManager(t)
	if _, err := manager.Join(context.Background(), ""); err == nil {
		t.Fatal("playerIDなしで参加できました")
	}
}

// runtime.Runnerを満たしていることをcompile時に確認する。
var _ runtime.Runner = (*runtimetest.Fake)(nil)
