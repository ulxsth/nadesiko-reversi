package protocol_test

import (
	"testing"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

func TestRequestValidate(t *testing.T) {
	state := newPlayingState()
	seed := uint32(1)

	tests := []struct {
		name    string
		request protocol.Request
		want    protocol.ErrorCode
	}{
		{
			name:    "正常なnewGame",
			request: protocol.NewGameRequest("ok", 1),
		},
		{
			name:    "正常なapplyCommand",
			request: protocol.ApplyCommandRequest(state, protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2)),
		},
		{
			name:    "version欠落",
			request: protocol.Request{Action: protocol.ActionNewGame, GameID: "x", Seed: &seed},
			want:    protocol.CodeUnsupportedVersion,
		},
		{
			name:    "version不一致",
			request: protocol.Request{Version: "2", Action: protocol.ActionNewGame, GameID: "x", Seed: &seed},
			want:    protocol.CodeUnsupportedVersion,
		},
		{
			name:    "未知のaction",
			request: protocol.Request{Version: protocol.Version, Action: "restart"},
			want:    protocol.CodeInvalidRequest,
		},
		{
			name:    "newGameにgameIdがない",
			request: protocol.Request{Version: protocol.Version, Action: protocol.ActionNewGame, Seed: &seed},
			want:    protocol.CodeInvalidRequest,
		},
		{
			name:    "newGameにseedがない",
			request: protocol.Request{Version: protocol.Version, Action: protocol.ActionNewGame, GameID: "x"},
			want:    protocol.CodeInvalidRequest,
		},
		{
			name:    "newGameにstateが混ざる",
			request: protocol.Request{Version: protocol.Version, Action: protocol.ActionNewGame, GameID: "x", Seed: &seed, State: &state},
			want:    protocol.CodeInvalidRequest,
		},
		{
			name:    "applyCommandにstateがない",
			request: protocol.Request{Version: protocol.Version, Action: protocol.ActionApplyCommand, Command: &protocol.Command{}},
			want:    protocol.CodeInvalidRequest,
		},
		{
			name:    "applyCommandにcommandがない",
			request: protocol.Request{Version: protocol.Version, Action: protocol.ActionApplyCommand, State: &state},
			want:    protocol.CodeInvalidRequest,
		},
		{
			name:    "applyCommandにseedが混ざる",
			request: protocol.Request{Version: protocol.Version, Action: protocol.ActionApplyCommand, State: &state, Command: &protocol.Command{}, Seed: &seed},
			want:    protocol.CodeInvalidRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			assertCode(t, err, tt.want)
		})
	}
}

func TestCommandValidate(t *testing.T) {
	outside := 8
	zero := 0

	tests := []struct {
		name    string
		command protocol.Command
		want    protocol.ErrorCode
	}{
		{
			name:    "正常なplace",
			command: protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 0, 0),
		},
		{
			name:    "正常なpass",
			command: protocol.PassCommand("cmd-2", protocol.PlayerLight, 4),
		},
		{
			name:    "commandIdがない",
			command: protocol.Command{Type: protocol.CommandPass, Player: protocol.PlayerDark},
			want:    protocol.CodeInvalidCommand,
		},
		{
			name:    "playerが不正",
			command: protocol.Command{CommandID: "c", Type: protocol.CommandPass, Player: "green"},
			want:    protocol.CodeInvalidCommand,
		},
		{
			name:    "expectedTurnが負",
			command: protocol.Command{CommandID: "c", Type: protocol.CommandPass, Player: protocol.PlayerDark, ExpectedTurn: -1},
			want:    protocol.CodeInvalidCommand,
		},
		{
			name:    "未知のtype",
			command: protocol.Command{CommandID: "c", Type: "resign", Player: protocol.PlayerDark},
			want:    protocol.CodeInvalidCommand,
		},
		{
			name:    "placeに座標がない",
			command: protocol.Command{CommandID: "c", Type: protocol.CommandPlace, Player: protocol.PlayerDark},
			want:    protocol.CodeInvalidCommand,
		},
		{
			name:    "placeの座標が盤外",
			command: protocol.Command{CommandID: "c", Type: protocol.CommandPlace, Player: protocol.PlayerDark, Row: &outside, Col: &zero},
			want:    protocol.CodeInvalidCoordinate,
		},
		{
			name:    "passに座標が混ざる",
			command: protocol.Command{CommandID: "c", Type: protocol.CommandPass, Player: protocol.PlayerDark, Row: &zero, Col: &zero},
			want:    protocol.CodeInvalidCommand,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCode(t, tt.command.Validate(), tt.want)
		})
	}
}

func TestStateValidate(t *testing.T) {
	dark := protocol.PlayerDark
	invalidPlayer := protocol.Player("grey")

	tests := []struct {
		name   string
		mutate func(*protocol.State)
		want   protocol.ErrorCode
	}{
		{
			name:   "正常なplaying",
			mutate: func(*protocol.State) {},
		},
		{
			name:   "version不一致",
			mutate: func(s *protocol.State) { s.Version = "0" },
			want:   protocol.CodeUnsupportedVersion,
		},
		{
			name:   "gameIdがない",
			mutate: func(s *protocol.State) { s.GameID = "" },
			want:   protocol.CodeInvalidState,
		},
		{
			name:   "boardが64マスでない",
			mutate: func(s *protocol.State) { s.Board = s.Board[:63] },
			want:   protocol.CodeInvalidState,
		},
		{
			name:   "turnNumberが負",
			mutate: func(s *protocol.State) { s.TurnNumber = -1 },
			want:   protocol.CodeInvalidState,
		},
		{
			name:   "currentPlayerが不正",
			mutate: func(s *protocol.State) { s.CurrentPlayer = "grey" },
			want:   protocol.CodeInvalidState,
		},
		{
			name:   "consecutivePassesが範囲外",
			mutate: func(s *protocol.State) { s.ConsecutivePasses = 3 },
			want:   protocol.CodeInvalidState,
		},
		{
			name:   "phaseが不正",
			mutate: func(s *protocol.State) { s.Phase = "paused" },
			want:   protocol.CodeInvalidState,
		},
		{
			name:   "legalMovesに盤外座標",
			mutate: func(s *protocol.State) { s.LegalMoves[0] = protocol.Position{Row: 8, Col: 0} },
			want:   protocol.CodeInvalidState,
		},
		{
			name:   "playing中にwinnerがある",
			mutate: func(s *protocol.State) { s.Winner = &dark },
			want:   protocol.CodeInvalidState,
		},
		{
			name: "正常なfinished",
			mutate: func(s *protocol.State) {
				s.Phase = protocol.PhaseFinished
				s.Winner = &dark
				s.LegalMoves = nil
			},
		},
		{
			name: "finishedにwinnerがない",
			mutate: func(s *protocol.State) {
				s.Phase = protocol.PhaseFinished
				s.LegalMoves = nil
			},
			want: protocol.CodeInvalidState,
		},
		{
			name: "finishedのwinnerが不正",
			mutate: func(s *protocol.State) {
				s.Phase = protocol.PhaseFinished
				s.Winner = &invalidPlayer
				s.LegalMoves = nil
			},
			want: protocol.CodeInvalidState,
		},
		{
			name: "finishedにlegalMovesが残る",
			mutate: func(s *protocol.State) {
				s.Phase = protocol.PhaseFinished
				s.Winner = &dark
			},
			want: protocol.CodeInvalidState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newPlayingState()
			tt.mutate(&state)
			assertCode(t, state.Validate(), tt.want)
		})
	}
}

func TestResponseValidate(t *testing.T) {
	state := newPlayingState()
	row, col := 2, 2
	color := uint8(60)

	placed := protocol.Event{
		CommandID:   "cmd-1",
		Type:        protocol.EventPlaced,
		Player:      protocol.PlayerDark,
		TurnNumber:  1,
		Row:         &row,
		Col:         &col,
		PlacedColor: &color,
		Changes:     []protocol.Change{{Row: 3, Col: 3, Color: 20}},
	}
	passed := protocol.Event{
		CommandID:  "cmd-2",
		Type:       protocol.EventPassed,
		Player:     protocol.PlayerLight,
		TurnNumber: 2,
	}

	tests := []struct {
		name     string
		response protocol.Response
		want     protocol.ErrorCode
	}{
		{
			name:     "成功応答",
			response: protocol.Response{Version: protocol.Version, OK: true, State: &state},
		},
		{
			name:     "placed event付き成功応答",
			response: protocol.Response{Version: protocol.Version, OK: true, State: &state, Event: &placed},
		},
		{
			name:     "passed event付き成功応答",
			response: protocol.Response{Version: protocol.Version, OK: true, State: &state, Event: &passed},
		},
		{
			name:     "拒否応答",
			response: protocol.Response{Version: protocol.Version, OK: false, Error: protocol.NewError(protocol.CodeOccupied, "そのマスには既に駒があります"), State: &state},
		},
		{
			name:     "version不一致",
			response: protocol.Response{Version: "9", OK: true, State: &state},
			want:     protocol.CodeUnsupportedVersion,
		},
		{
			name:     "成功応答にstateがない",
			response: protocol.Response{Version: protocol.Version, OK: true},
			want:     protocol.CodeInvalidState,
		},
		{
			name:     "成功応答にerrorが混ざる",
			response: protocol.Response{Version: protocol.Version, OK: true, State: &state, Error: protocol.NewError(protocol.CodeOccupied, "x")},
			want:     protocol.CodeInvalidRequest,
		},
		{
			name:     "拒否応答にerrorがない",
			response: protocol.Response{Version: protocol.Version, OK: false},
			want:     protocol.CodeInvalidRequest,
		},
		{
			name:     "拒否応答のcodeが未知",
			response: protocol.Response{Version: protocol.Version, OK: false, Error: protocol.NewError("boom", "x")},
			want:     protocol.CodeInvalidRequest,
		},
		{
			name:     "拒否応答にmessageがない",
			response: protocol.Response{Version: protocol.Version, OK: false, Error: protocol.NewError(protocol.CodeOccupied, "")},
			want:     protocol.CodeInvalidRequest,
		},
		{
			name:     "拒否応答にeventが混ざる",
			response: protocol.Response{Version: protocol.Version, OK: false, Error: protocol.NewError(protocol.CodeOccupied, "x"), Event: &placed},
			want:     protocol.CodeInvalidRequest,
		},
		{
			name: "passed eventに着手情報が混ざる",
			response: protocol.Response{Version: protocol.Version, OK: true, State: &state, Event: &protocol.Event{
				CommandID: "cmd-3", Type: protocol.EventPassed, Player: protocol.PlayerDark, TurnNumber: 1, Row: &row, Col: &col,
			}},
			want: protocol.CodeInvalidCommand,
		},
		{
			name: "placed eventにplacedColorがない",
			response: protocol.Response{Version: protocol.Version, OK: true, State: &state, Event: &protocol.Event{
				CommandID: "cmd-4", Type: protocol.EventPlaced, Player: protocol.PlayerDark, TurnNumber: 1, Row: &row, Col: &col,
			}},
			want: protocol.CodeInvalidCommand,
		},
		{
			name: "eventのturnNumberが0",
			response: protocol.Response{Version: protocol.Version, OK: true, State: &state, Event: &protocol.Event{
				CommandID: "cmd-5", Type: protocol.EventPassed, Player: protocol.PlayerDark, TurnNumber: 0,
			}},
			want: protocol.CodeInvalidCommand,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCode(t, tt.response.Validate(), tt.want)
		})
	}
}

// assertCode はvalidationの結果が期待するcodeかどうかを確認する。
// wantが空文字のときはerrorが返らないことを期待する。
func assertCode(t *testing.T, err *protocol.Error, want protocol.ErrorCode) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("エラーを期待していませんでした: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("%qを期待しましたがエラーがありません", want)
	}
	if err.Code != want {
		t.Fatalf("code が違います: got %q, want %q (%s)", err.Code, want, err.Message)
	}
}
