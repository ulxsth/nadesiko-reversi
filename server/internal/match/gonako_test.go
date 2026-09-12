package match_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ulxsth/nadesiko-reversi/server/internal/match"
	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

func repoPath(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
}

// newGonakoManager は実ルールengineを使うmanagerを作る。
// gonakoが未配置の環境ではskipし、make bootstrap後のCIでだけ実行する。
func newGonakoManager(t *testing.T) *match.Manager {
	t.Helper()

	binary := os.Getenv("GONAKO_BIN")
	if binary == "" {
		binary = repoPath(".tools", "bin", "gonako")
	}
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("gonakoが見つからないためskipします (%s): make bootstrapが必要です", binary)
	}

	runner, err := runtime.New(runtime.Config{
		GonakoPath: binary,
		RulesPath:  repoPath("rules", "game", "main.nako3"),
		Timeout:    30 * time.Second,
	})
	if err != nil {
		t.Fatalf("rule runnerを作れません: %v", err)
	}

	manager, err := match.NewManager(runner, match.WithSeed(1))
	if err != nil {
		t.Fatalf("managerを作れません: %v", err)
	}
	t.Cleanup(func() { manager.Close("test終了") })
	return manager
}

// seatOf は席に対応する参加情報を返す。
func seatOf(first, second *match.Membership, player protocol.Player) *match.Membership {
	if first.Seat == player {
		return first
	}
	return second
}

func TestGonakoMatchPlaysRealRules(t *testing.T) {
	ctx := context.Background()
	manager := newGonakoManager(t)

	alice, err := manager.Join(ctx, "alice")
	if err != nil {
		t.Fatalf("参加に失敗: %v", err)
	}
	bob, err := manager.Join(ctx, "bob")
	if err != nil {
		t.Fatalf("参加に失敗: %v", err)
	}

	matched := expectEvent(t, alice.Events, match.EventMatched)
	expectEvent(t, bob.Events, match.EventMatched)

	state := matched.State
	if state == nil {
		t.Fatal("成立eventにstateがありません")
	}
	// 契約の固定vector: seed=1 の初期state
	if state.NextColor != 60 || state.RNGState != 1015568748 {
		t.Errorf("初期stateが契約と違います: nextColor=%d rngState=%d", state.NextColor, state.RNGState)
	}
	if len(state.LegalMoves) != 12 {
		t.Errorf("初期の合法手が違います: %d", len(state.LegalMoves))
	}

	// 手番でない側のcommandは拒否される
	light := seatOf(alice, bob, protocol.PlayerLight)
	notYourTurn, err := manager.Submit(ctx, light.RoomID, light.PlayerID,
		protocol.PlaceCommand("cmd-early", protocol.PlayerLight, 0, 2, 2))
	if err != nil {
		t.Fatalf("評価に失敗: %v", err)
	}
	if notYourTurn.OK || notYourTurn.Error.Code != protocol.CodeNotYourTurn {
		t.Errorf("手番違いが拒否されません: %+v", notYourTurn)
	}

	// 先手が合法手を置く
	dark := seatOf(alice, bob, protocol.PlayerDark)
	move := state.LegalMoves[0]
	accepted, err := manager.Submit(ctx, dark.RoomID, dark.PlayerID,
		protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, move.Row, move.Col))
	if err != nil {
		t.Fatalf("着手に失敗: %v", err)
	}
	if !accepted.OK {
		t.Fatalf("合法手が拒否されました: %+v", accepted.Error)
	}
	if accepted.Event == nil || accepted.Event.PlacedColor == nil {
		t.Fatalf("eventに着手情報がありません: %+v", accepted.Event)
	}
	if *accepted.Event.PlacedColor != 60 {
		t.Errorf("置いた駒色が違います: %d", *accepted.Event.PlacedColor)
	}

	// 両者へ確定state全体と次の駒色が届く
	for name, events := range map[string]<-chan match.Event{
		"dark": dark.Events, "light": light.Events,
	} {
		event := expectEvent(t, events, match.EventState)
		if event.State == nil {
			t.Errorf("%s: stateが届きません", name)
			continue
		}
		if event.State.TurnNumber != 1 {
			t.Errorf("%s: turnNumberが違います: %d", name, event.State.TurnNumber)
		}
		if event.State.CurrentPlayer != protocol.PlayerLight {
			t.Errorf("%s: 次手番が違います: %q", name, event.State.CurrentPlayer)
		}
		if event.State.NextColor != 94 {
			t.Errorf("%s: 次の駒色が違います: %d", name, event.State.NextColor)
		}
		if event.State.Phase != protocol.PhasePlaying {
			t.Errorf("%s: phaseが違います: %q", name, event.State.Phase)
		}
		if cell := event.State.Board.At(move.Row, move.Col); cell == nil || *cell != 60 {
			t.Errorf("%s: 盤面に着手が反映されていません", name)
		}
	}

	// 同じexpectedTurnの再送はstale_turn
	stale, err := manager.Submit(ctx, dark.RoomID, dark.PlayerID,
		protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, move.Row, move.Col))
	if err != nil {
		t.Fatalf("再送の評価に失敗: %v", err)
	}
	if stale.OK || stale.Error.Code != protocol.CodeStaleTurn {
		t.Errorf("再送がstale_turnになりません: %+v", stale)
	}
	if stale.State == nil || stale.State.TurnNumber != 1 {
		t.Errorf("拒否で盤面が動きました: %+v", stale.State)
	}
}

func TestGonakoMatchReachesFinish(t *testing.T) {
	ctx := context.Background()
	manager := newGonakoManager(t)

	alice, _ := manager.Join(ctx, "alice")
	bob, _ := manager.Join(ctx, "bob")
	matched := expectEvent(t, alice.Events, match.EventMatched)
	expectEvent(t, bob.Events, match.EventMatched)

	// 配信が詰まらないよう読み続ける
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-bob.Events:
			case <-stop:
				return
			}
		}
	}()
	defer close(stop)

	state := *matched.State
	var closed bool

	for turn := 0; turn < 200 && state.Phase == protocol.PhasePlaying; turn++ {
		seat := seatOf(alice, bob, state.CurrentPlayer)

		var command protocol.Command
		if len(state.LegalMoves) == 0 {
			command = protocol.PassCommand(fmt.Sprintf("cmd-%d", turn+1), state.CurrentPlayer, state.TurnNumber)
		} else {
			move := state.LegalMoves[turn%len(state.LegalMoves)]
			command = protocol.PlaceCommand(fmt.Sprintf("cmd-%d", turn+1), state.CurrentPlayer, state.TurnNumber, move.Row, move.Col)
		}

		response, err := manager.Submit(ctx, seat.RoomID, seat.PlayerID, command)
		if err != nil {
			t.Fatalf("%d手目に失敗: %v", turn+1, err)
		}
		if !response.OK || response.State == nil {
			t.Fatalf("%d手目が拒否されました: %+v", turn+1, response.Error)
		}
		state = *response.State

		// aliceの配信を読み進め、終了eventを拾う
		for {
			select {
			case event, ok := <-alice.Events:
				if !ok {
					closed = true
				} else if event.Type == match.EventClosed {
					closed = true
				} else {
					continue
				}
			default:
			}
			break
		}
	}

	if state.Phase != protocol.PhaseFinished {
		t.Fatalf("対局が終わりません: %s", state.Phase)
	}
	if state.Winner == nil {
		t.Fatal("勝者がありません")
	}

	room, ok := manager.Room(alice.RoomID)
	if !ok {
		t.Fatal("roomが消えました")
	}
	if room.Phase() != match.PhaseClosed {
		t.Errorf("終局でroomが閉じません: %q", room.Phase())
	}
	if !closed {
		// 読み取りの取りこぼしはあり得るので、room側の状態で判断する
		t.Logf("終了eventは読み取れなかったが、roomはclosed")
	}

	// 終局後のcommandは拒否される
	dark := seatOf(alice, bob, protocol.PlayerDark)
	after, err := manager.Submit(ctx, dark.RoomID, dark.PlayerID,
		protocol.PlaceCommand("cmd-after", protocol.PlayerDark, state.TurnNumber, 0, 0))
	if err != nil {
		t.Fatalf("評価に失敗: %v", err)
	}
	if after.OK {
		t.Error("終局後のcommandが受理されました")
	}
}
