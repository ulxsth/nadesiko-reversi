package runtime_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

// repoPath はpackage directoryからrepository rootの相対パスを組み立てる。
func repoPath(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
}

// gonakoPath は検証に使うgonakoの位置を返す。
// bootstrapが配置する.tools/bin/gonakoを既定とし、GONAKO_BINで上書きできる。
func gonakoPath() string {
	if custom := os.Getenv("GONAKO_BIN"); custom != "" {
		return custom
	}
	return repoPath(".tools", "bin", "gonako")
}

// newGonako は実際のルールengineを呼ぶrunnerを作る。
// gonakoが未配置の環境ではskipし、make bootstrap後のCIでだけ実行する。
func newGonako(t *testing.T, timeout time.Duration) *runtime.Gonako {
	t.Helper()

	binary := gonakoPath()
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("gonakoが見つからないためskipします (%s): make bootstrapが必要です", binary)
	}

	runner, err := runtime.New(runtime.Config{
		GonakoPath: binary,
		RulesPath:  repoPath("rules", "game", "main.nako3"),
		Timeout:    timeout,
	})
	if err != nil {
		t.Fatalf("runnerを作れません: %v", err)
	}
	return runner
}

// newStubRunner はtestdataのstubをルールengineとして実行するrunnerを作る。
func newStubRunner(t *testing.T, stub string) *runtime.Gonako {
	t.Helper()

	binary := gonakoPath()
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("gonakoが見つからないためskipします (%s): make bootstrapが必要です", binary)
	}

	runner, err := runtime.New(runtime.Config{
		GonakoPath: binary,
		RulesPath:  filepath.Join("testdata", stub),
		Timeout:    30 * time.Second,
	})
	if err != nil {
		t.Fatalf("runnerを作れません: %v", err)
	}
	return runner
}

// startGame はseed付きの新規ゲームを作り、そのstateを返す。
func startGame(t *testing.T, runner runtime.Runner, gameID string, seed uint32) protocol.State {
	t.Helper()

	response, err := runtime.NewGame(context.Background(), runner, gameID, seed)
	if err != nil {
		t.Fatalf("newGameに失敗: %v", err)
	}
	if !response.OK || response.State == nil {
		t.Fatalf("newGameが拒否されました: %+v", response.Error)
	}
	return *response.State
}

func TestGonakoNewGameMatchesContractVector(t *testing.T) {
	runner := newGonako(t, 30*time.Second)

	state := startGame(t, runner, "fixture-new-game", 1)

	if state.Version != protocol.Version {
		t.Errorf("versionが違います: %q", state.Version)
	}
	if state.GameID != "fixture-new-game" {
		t.Errorf("gameIdが違います: %q", state.GameID)
	}
	if state.NextColor != 60 {
		t.Errorf("nextColorが違います: got %d, want 60", state.NextColor)
	}
	if state.RNGState != 1015568748 {
		t.Errorf("rngStateが違います: got %d, want 1015568748", state.RNGState)
	}
	if state.TurnNumber != 0 {
		t.Errorf("turnNumberが違います: %d", state.TurnNumber)
	}
	if state.CurrentPlayer != protocol.PlayerDark {
		t.Errorf("先手が違います: %q", state.CurrentPlayer)
	}
	if state.Phase != protocol.PhasePlaying {
		t.Errorf("phaseが違います: %q", state.Phase)
	}
	if state.Winner != nil {
		t.Errorf("playing中にwinnerがあります: %v", *state.Winner)
	}

	// 契約の初期配置: (3,3)=0、(4,4)=0、(3,4)=255、(4,3)=255
	for _, want := range []struct {
		row, col int
		color    uint8
	}{{3, 3, 0}, {4, 4, 0}, {3, 4, 255}, {4, 3, 255}} {
		cell := state.Board.At(want.row, want.col)
		if cell == nil {
			t.Errorf("(%d,%d)が空きマスです", want.row, want.col)
			continue
		}
		if *cell != want.color {
			t.Errorf("(%d,%d)の色が違います: got %d, want %d", want.row, want.col, *cell, want.color)
		}
	}

	wantMoves := []protocol.Position{
		{Row: 2, Col: 2}, {Row: 2, Col: 3}, {Row: 2, Col: 4}, {Row: 2, Col: 5},
		{Row: 3, Col: 2}, {Row: 3, Col: 5}, {Row: 4, Col: 2}, {Row: 4, Col: 5},
		{Row: 5, Col: 2}, {Row: 5, Col: 3}, {Row: 5, Col: 4}, {Row: 5, Col: 5},
	}
	if len(state.LegalMoves) != len(wantMoves) {
		t.Fatalf("合法手の件数が違います: got %d, want %d", len(state.LegalMoves), len(wantMoves))
	}
	for i, want := range wantMoves {
		if state.LegalMoves[i] != want {
			t.Errorf("合法手[%d]が違います: got %+v, want %+v", i, state.LegalMoves[i], want)
		}
	}
}

func TestGonakoPlaceConvertsSandwichedPiece(t *testing.T) {
	runner := newGonako(t, 30*time.Second)
	state := startGame(t, runner, "place-vector", 1)

	response, err := runtime.ApplyCommand(context.Background(), runner, state,
		protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2))
	if err != nil {
		t.Fatalf("applyCommandに失敗: %v", err)
	}
	if !response.OK {
		t.Fatalf("着手が拒否されました: %+v", response.Error)
	}

	// 契約の例: nextColor=60を(2,2)へ置くと(3,3)がfloor((60+0+0)/3)=20になる。
	if cell := response.State.Board.At(2, 2); cell == nil || *cell != 60 {
		t.Errorf("(2,2)へ置いた駒色が違います: %v", cell)
	}
	if cell := response.State.Board.At(3, 3); cell == nil || *cell != 20 {
		t.Errorf("(3,3)の変換後の色が違います: %v", cell)
	}
	if response.State.TurnNumber != 1 {
		t.Errorf("turnNumberが進んでいません: %d", response.State.TurnNumber)
	}
	if response.State.CurrentPlayer != protocol.PlayerLight {
		t.Errorf("手番が交代していません: %q", response.State.CurrentPlayer)
	}
	if response.State.RNGState == state.RNGState {
		t.Error("受理された着手で乱数stateが進んでいません")
	}

	event := response.Event
	if event == nil {
		t.Fatal("成功応答にeventがありません")
	}
	if event.Type != protocol.EventPlaced || event.CommandID != "cmd-1" {
		t.Errorf("eventの識別子が違います: %+v", event)
	}
	if event.Player != protocol.PlayerDark || event.TurnNumber != 1 {
		t.Errorf("eventの手番情報が違います: %+v", event)
	}
	if event.Row == nil || *event.Row != 2 || event.Col == nil || *event.Col != 2 {
		t.Errorf("eventの座標が違います: %+v", event)
	}
	if event.PlacedColor == nil || *event.PlacedColor != 60 {
		t.Errorf("placedColorが違います: %v", event.PlacedColor)
	}
	want := []protocol.Change{{Row: 3, Col: 3, Color: 20}}
	if len(event.Changes) != len(want) || event.Changes[0] != want[0] {
		t.Errorf("changesが違います: got %+v, want %+v", event.Changes, want)
	}
}

func TestGonakoRejectsInvalidCommandsWithoutMutatingState(t *testing.T) {
	runner := newGonako(t, 30*time.Second)
	state := startGame(t, runner, "reject-vector", 1)

	tests := []struct {
		name    string
		command protocol.Command
		want    protocol.ErrorCode
	}{
		{
			name:    "占有マスへの着手",
			command: protocol.PlaceCommand("cmd-occupied", protocol.PlayerDark, 0, 3, 3),
			want:    protocol.CodeOccupied,
		},
		{
			name:    "挟みが成立しない着手",
			command: protocol.PlaceCommand("cmd-illegal", protocol.PlayerDark, 0, 0, 0),
			want:    protocol.CodeIllegalMove,
		},
		{
			name:    "手番ではないplayer",
			command: protocol.PlaceCommand("cmd-turn", protocol.PlayerLight, 0, 2, 2),
			want:    protocol.CodeNotYourTurn,
		},
		{
			name:    "古いturn",
			command: protocol.PlaceCommand("cmd-stale", protocol.PlayerDark, 5, 2, 2),
			want:    protocol.CodeStaleTurn,
		},
		{
			name:    "合法手があるのにpass",
			command: protocol.PassCommand("cmd-pass", protocol.PlayerDark, 0),
			want:    protocol.CodePassNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := runtime.ApplyCommand(context.Background(), runner, state, tt.command)
			if err != nil {
				t.Fatalf("applyCommandに失敗: %v", err)
			}
			assertRejected(t, response, tt.want)

			if response.State == nil {
				t.Fatal("拒否応答に変更前stateがありません")
			}
			if response.State.TurnNumber != state.TurnNumber {
				t.Errorf("拒否でturnNumberが動きました: %d", response.State.TurnNumber)
			}
			if response.State.CurrentPlayer != state.CurrentPlayer {
				t.Errorf("拒否で手番が動きました: %q", response.State.CurrentPlayer)
			}
			if response.State.RNGState != state.RNGState {
				t.Errorf("拒否で乱数stateが動きました: %d", response.State.RNGState)
			}
			if response.State.NextColor != state.NextColor {
				t.Errorf("拒否でnextColorが動きました: %d", response.State.NextColor)
			}
			if response.State.ConsecutivePasses != state.ConsecutivePasses {
				t.Errorf("拒否でconsecutivePassesが動きました: %d", response.State.ConsecutivePasses)
			}
			if cell := response.State.Board.At(2, 2); cell != nil {
				t.Errorf("拒否で盤面が変わりました: (2,2)=%d", *cell)
			}
		})
	}
}

// 同時呼び出しでstdinとstdoutが混線しないことを確認する。
// 混線すればgameIdや決定的な乱数stateが取り違えられる。
func TestGonakoConcurrentEvaluationsStayIsolated(t *testing.T) {
	runner := newGonako(t, 30*time.Second)

	const workers = 8

	type result struct {
		gameID   string
		rngState uint32
		color    uint8
	}

	var mu sync.Mutex
	results := make(map[string]result, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()

			gameID := fmt.Sprintf("concurrent-%d", i)
			response, err := runtime.NewGame(context.Background(), runner, gameID, uint32(i))
			if err != nil {
				t.Errorf("%s: newGameに失敗: %v", gameID, err)
				return
			}
			if !response.OK || response.State == nil {
				t.Errorf("%s: newGameが拒否されました: %+v", gameID, response.Error)
				return
			}
			if response.State.GameID != gameID {
				t.Errorf("gameIdが混線しました: got %q, want %q", response.State.GameID, gameID)
				return
			}

			mu.Lock()
			results[gameID] = result{
				gameID:   response.State.GameID,
				rngState: response.State.RNGState,
				color:    response.State.NextColor,
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(results) != workers {
		t.Fatalf("結果件数が違います: got %d, want %d", len(results), workers)
	}

	// 同じseedを直列で評価し直し、並行実行が値を壊していないことを確認する。
	for i := 0; i < workers; i++ {
		gameID := fmt.Sprintf("concurrent-%d", i)
		state := startGame(t, runner, gameID, uint32(i))

		got := results[gameID]
		if got.rngState != state.RNGState || got.color != state.NextColor {
			t.Errorf("%s: 並行実行の結果が直列実行と違います: got {rng:%d color:%d}, want {rng:%d color:%d}",
				gameID, got.rngState, got.color, state.RNGState, state.NextColor)
		}
	}
}

func TestGonakoTimeoutIsTyped(t *testing.T) {
	runner := newGonako(t, time.Nanosecond)

	_, err := runtime.NewGame(context.Background(), runner, "timeout", 1)
	if err == nil {
		t.Fatal("TimeoutErrorを期待しましたが成功しました")
	}

	var timeoutErr *runtime.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("TimeoutErrorではありません: %T %v", err, err)
	}
	if timeoutErr.Timeout != time.Nanosecond {
		t.Errorf("Timeoutが記録されていません: %s", timeoutErr.Timeout)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("context.DeadlineExceededをUnwrapできません")
	}
}

func TestGonakoCanceledContextPropagates(t *testing.T) {
	runner := newGonako(t, 30*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runtime.NewGame(ctx, runner, "canceled", 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("context.Canceledを期待しました: %T %v", err, err)
	}
}

func TestGonakoExitFailureIsTyped(t *testing.T) {
	runner := newStubRunner(t, "exit-failure.nako3")

	_, err := runtime.NewGame(context.Background(), runner, "exit", 1)
	if err == nil {
		t.Fatal("ExecErrorを期待しましたが成功しました")
	}

	var execErr *runtime.ExecError
	if !errors.As(err, &execErr) {
		t.Fatalf("ExecErrorではありません: %T %v", err, err)
	}
	if execErr.ExitCode == 0 {
		t.Errorf("終了コードが記録されていません: %d", execErr.ExitCode)
	}
	if execErr.Stderr == "" {
		t.Error("stderr診断が記録されていません")
	}
	if code := execErr.ProtocolError().Code; code != protocol.CodeInvalidJSON {
		t.Errorf("契約codeへの変換が違います: %q", code)
	}
}

func TestGonakoNonJSONOutputIsTyped(t *testing.T) {
	runner := newStubRunner(t, "not-json.nako3")

	_, err := runtime.NewGame(context.Background(), runner, "not-json", 1)
	if err == nil {
		t.Fatal("DecodeErrorを期待しましたが成功しました")
	}

	var decodeErr *runtime.DecodeError
	if !errors.As(err, &decodeErr) {
		t.Fatalf("DecodeErrorではありません: %T %v", err, err)
	}
	if decodeErr.Output == "" {
		t.Error("解釈できなかった出力が記録されていません")
	}
}

func TestGonakoContractViolationIsRejected(t *testing.T) {
	runner := newStubRunner(t, "contract-violation.nako3")

	_, err := runtime.NewGame(context.Background(), runner, "violation", 1)
	if err == nil {
		t.Fatal("DecodeErrorを期待しましたが成功しました")
	}

	var decodeErr *runtime.DecodeError
	if !errors.As(err, &decodeErr) {
		t.Fatalf("DecodeErrorではありません: %T %v", err, err)
	}

	var protocolErr *protocol.Error
	if !errors.As(err, &protocolErr) {
		t.Fatalf("契約違反の理由をUnwrapできません: %v", err)
	}
	if protocolErr.Code != protocol.CodeInvalidState {
		t.Errorf("契約違反のcodeが違います: %q", protocolErr.Code)
	}
}
