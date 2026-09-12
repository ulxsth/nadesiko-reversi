package runtime_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime/runtimetest"
)

func TestNewRejectsInvalidConfig(t *testing.T) {
	existing := filepath.Join("testdata", "not-json.nako3")

	tests := []struct {
		name   string
		config runtime.Config
	}{
		{name: "gonakoPathが空", config: runtime.Config{RulesPath: existing}},
		{name: "rulesPathが空", config: runtime.Config{GonakoPath: existing}},
		{name: "gonakoが存在しない", config: runtime.Config{GonakoPath: filepath.Join("testdata", "missing"), RulesPath: existing}},
		{name: "ルールが存在しない", config: runtime.Config{GonakoPath: existing, RulesPath: filepath.Join("testdata", "missing.nako3")}},
		{name: "gonakoがディレクトリ", config: runtime.Config{GonakoPath: "testdata", RulesPath: existing}},
		{name: "ルールがディレクトリ", config: runtime.Config{GonakoPath: existing, RulesPath: "testdata"}},
		{name: "Timeoutが負", config: runtime.Config{GonakoPath: existing, RulesPath: existing, Timeout: -time.Second}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := runtime.New(tt.config)
			if err == nil {
				t.Fatalf("ConfigErrorを期待しましたが成功しました: %+v", runner)
			}
			var configErr *runtime.ConfigError
			if !errors.As(err, &configErr) {
				t.Fatalf("ConfigErrorではありません: %T %v", err, err)
			}
		})
	}
}

func TestNewAppliesDefaultTimeout(t *testing.T) {
	stub := filepath.Join("testdata", "not-json.nako3")

	runner, err := runtime.New(runtime.Config{GonakoPath: stub, RulesPath: stub})
	if err != nil {
		t.Fatalf("runnerを作れません: %v", err)
	}
	if got := runner.Timeout(); got != runtime.DefaultTimeout {
		t.Errorf("既定のTimeoutが違います: got %s, want %s", got, runtime.DefaultTimeout)
	}
	if !runner.Ready() {
		t.Error("Readyがfalseを返しました")
	}
}

// 不正なrequestはprocessを起動せずに拒否する。
// 起動していればexit-failure stubがExecErrorになるため、拒否responseが返れば未起動と分かる。
func TestEvaluateRejectsInvalidRequestWithoutRunningProcess(t *testing.T) {
	stub := filepath.Join("testdata", "exit-failure.nako3")
	runner, err := runtime.New(runtime.Config{GonakoPath: stub, RulesPath: stub})
	if err != nil {
		t.Fatalf("runnerを作れません: %v", err)
	}

	tests := []struct {
		name    string
		request protocol.Request
		want    protocol.ErrorCode
	}{
		{
			name:    "version不一致",
			request: protocol.Request{Version: "2", Action: protocol.ActionNewGame, GameID: "x"},
			want:    protocol.CodeUnsupportedVersion,
		},
		{
			name:    "seedがない",
			request: protocol.Request{Version: protocol.Version, Action: protocol.ActionNewGame, GameID: "x"},
			want:    protocol.CodeInvalidRequest,
		},
		{
			name:    "未知のaction",
			request: protocol.Request{Version: protocol.Version, Action: "restart"},
			want:    protocol.CodeInvalidRequest,
		},
		{
			name: "座標が盤外",
			request: protocol.ApplyCommandRequest(
				runtimetest.NewState("local"),
				protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 8, 0),
			),
			want: protocol.CodeInvalidCoordinate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := runner.Evaluate(context.Background(), tt.request)
			if err != nil {
				t.Fatalf("processが起動したようです: %v", err)
			}
			assertRejected(t, response, tt.want)
		})
	}
}

func TestFakeRecordsRequests(t *testing.T) {
	fake := runtimetest.New(nil)

	if _, err := runtime.NewGame(context.Background(), fake, "game-1", 7); err != nil {
		t.Fatalf("NewGameに失敗: %v", err)
	}
	state := runtimetest.NewState("game-1")
	if _, err := runtime.ApplyCommand(context.Background(), fake, state, protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2)); err != nil {
		t.Fatalf("ApplyCommandに失敗: %v", err)
	}

	requests := fake.Requests()
	if len(requests) != 2 {
		t.Fatalf("記録件数が違います: %d", len(requests))
	}
	if requests[0].Action != protocol.ActionNewGame || requests[0].GameID != "game-1" {
		t.Errorf("newGame requestが記録されていません: %+v", requests[0])
	}
	if requests[0].Seed == nil || *requests[0].Seed != 7 {
		t.Errorf("seedが記録されていません: %+v", requests[0].Seed)
	}
	if requests[1].Action != protocol.ActionApplyCommand || requests[1].Command == nil {
		t.Errorf("applyCommand requestが記録されていません: %+v", requests[1])
	}

	fake.Reset()
	if fake.Len() != 0 {
		t.Errorf("Resetで記録が消えていません: %d", fake.Len())
	}
}

func TestFakeSubstitutesResponses(t *testing.T) {
	state := runtimetest.NewState("game-2")
	fake := runtimetest.New(func(_ context.Context, _ protocol.Request) (*protocol.Response, error) {
		return runtimetest.Rejected(protocol.CodeOccupied, "そのマスには既に駒があります", state), nil
	})

	response, err := runtime.NewGame(context.Background(), fake, "game-2", 1)
	if err != nil {
		t.Fatalf("評価に失敗: %v", err)
	}
	assertRejected(t, response, protocol.CodeOccupied)
	if err := response.Validate(); err != nil {
		t.Errorf("差し替えたresponseが契約を満たしません: %v", err)
	}
}

func TestFakeIsSafeForConcurrentUse(t *testing.T) {
	fake := runtimetest.New(nil)

	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if _, err := runtime.NewGame(context.Background(), fake, "concurrent", 1); err != nil {
				t.Errorf("評価に失敗: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := fake.Len(); got != workers {
		t.Errorf("記録件数が違います: got %d, want %d", got, workers)
	}
}

// assertRejected はresponseが指定codeの拒否であることを確認する。
func assertRejected(t *testing.T, response *protocol.Response, want protocol.ErrorCode) {
	t.Helper()
	if response == nil {
		t.Fatal("responseがnilです")
	}
	if response.OK {
		t.Fatalf("拒否を期待しましたが成功しました: %+v", response)
	}
	if response.Error == nil {
		t.Fatal("拒否responseにerrorがありません")
	}
	if response.Error.Code != want {
		t.Fatalf("error codeが違います: got %q, want %q (%s)", response.Error.Code, want, response.Error.Message)
	}
}
