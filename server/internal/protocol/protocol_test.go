package protocol_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

// contractExample はdocs/contracts/examplesの正本を読む。
// 契約を二重管理しないため、testdataへ複製せず参照する。
func contractExample(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "docs", "contracts", "examples", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("契約exampleを読めません %s: %v", path, err)
	}
	return data
}

// asGeneric はJSONを型なしの値へ落とし、意味的な比較を可能にする。
func asGeneric(t *testing.T, data []byte) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("JSONデコードに失敗: %v", err)
	}
	return value
}

func TestResponseRoundTripMatchesContractExamples(t *testing.T) {
	for _, name := range []string{"new-game-response.json", "occupied-response.json"} {
		t.Run(name, func(t *testing.T) {
			original := contractExample(t, name)

			var response protocol.Response
			if err := json.Unmarshal(original, &response); err != nil {
				t.Fatalf("responseをデコードできません: %v", err)
			}
			if err := response.Validate(); err != nil {
				t.Fatalf("契約exampleがvalidationに失敗: %v", err)
			}

			encoded, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("responseをエンコードできません: %v", err)
			}

			want := asGeneric(t, original)
			got := asGeneric(t, encoded)
			if !reflect.DeepEqual(want, got) {
				t.Errorf("往復で内容が変わりました\n元:   %s\n往復: %s", original, encoded)
			}
		})
	}
}

func TestRequestRoundTripMatchesContractExample(t *testing.T) {
	original := contractExample(t, "new-game-request.json")

	var request protocol.Request
	if err := json.Unmarshal(original, &request); err != nil {
		t.Fatalf("requestをデコードできません: %v", err)
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("契約exampleがvalidationに失敗: %v", err)
	}

	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("requestをエンコードできません: %v", err)
	}
	if !reflect.DeepEqual(asGeneric(t, original), asGeneric(t, encoded)) {
		t.Errorf("往復で内容が変わりました\n元:   %s\n往復: %s", original, encoded)
	}
}

func TestNewGameRequestKeepsZeroSeed(t *testing.T) {
	encoded, err := json.Marshal(protocol.NewGameRequest("zero-seed", 0))
	if err != nil {
		t.Fatalf("エンコードに失敗: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("デコードに失敗: %v", err)
	}
	seed, ok := decoded["seed"]
	if !ok {
		t.Fatalf("seed=0がエンコードで消えました: %s", encoded)
	}
	if seed != float64(0) {
		t.Errorf("seedが0ではありません: %v", seed)
	}
	if _, ok := decoded["state"]; ok {
		t.Errorf("newGameにstateが混ざりました: %s", encoded)
	}
}

func TestApplyCommandRequestOmitsNewGameFields(t *testing.T) {
	state := newPlayingState()
	request := protocol.ApplyCommandRequest(state, protocol.PlaceCommand("cmd-1", protocol.PlayerDark, 0, 2, 2))

	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("エンコードに失敗: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("デコードに失敗: %v", err)
	}
	for _, key := range []string{"gameId", "seed"} {
		if _, ok := decoded[key]; ok {
			t.Errorf("applyCommandに%sが混ざりました: %s", key, encoded)
		}
	}
	if err := request.Validate(); err != nil {
		t.Errorf("validationに失敗: %v", err)
	}
}

func TestPassCommandOmitsCoordinates(t *testing.T) {
	encoded, err := json.Marshal(protocol.PassCommand("cmd-pass", protocol.PlayerLight, 3))
	if err != nil {
		t.Fatalf("エンコードに失敗: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("デコードに失敗: %v", err)
	}
	for _, key := range []string{"row", "col"} {
		if _, ok := decoded[key]; ok {
			t.Errorf("passに%sが混ざりました: %s", key, encoded)
		}
	}
}

func TestStateAlwaysEncodesRequiredNullableFields(t *testing.T) {
	state := newPlayingState()
	state.LegalMoves = nil

	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("エンコードに失敗: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("デコードに失敗: %v", err)
	}

	moves, ok := decoded["legalMoves"]
	if !ok {
		t.Fatalf("legalMovesが消えました: %s", encoded)
	}
	if _, isSlice := moves.([]any); !isSlice {
		t.Errorf("legalMovesがarrayではありません: %v", moves)
	}

	winner, ok := decoded["winner"]
	if !ok {
		t.Fatalf("winnerが消えました: %s", encoded)
	}
	if winner != nil {
		t.Errorf("playing中のwinnerがnullではありません: %v", winner)
	}
}

func TestBoardCloneIsIndependent(t *testing.T) {
	state := newPlayingState()
	clone := state.Clone()

	*clone.Board[protocol.Index(3, 3)] = 7
	clone.Board[protocol.Index(0, 0)] = protocol.NewCell(1)
	clone.LegalMoves[0] = protocol.Position{Row: 7, Col: 7}

	if got := *state.Board[protocol.Index(3, 3)]; got != 0 {
		t.Errorf("複製の変更が元のマスへ伝播しました: %d", got)
	}
	if state.Board[protocol.Index(0, 0)] != nil {
		t.Error("複製の追加が元の空きマスへ伝播しました")
	}
	if state.LegalMoves[0] == (protocol.Position{Row: 7, Col: 7}) {
		t.Error("複製の変更が元のlegalMovesへ伝播しました")
	}
}

func TestBoardAtHandlesOutOfRange(t *testing.T) {
	board := newInitialBoard()

	if cell := board.At(3, 3); cell == nil || *cell != 0 {
		t.Errorf("(3,3)が初期駒ではありません: %v", cell)
	}
	if cell := board.At(-1, 0); cell != nil {
		t.Error("盤外が空きマスとして扱われていません")
	}
	if cell := board.At(8, 8); cell != nil {
		t.Error("盤外が空きマスとして扱われていません")
	}
}

func TestErrorCodesCoverContract(t *testing.T) {
	// docs/contracts/protocol.mdの表と同じ12件が揃っていることを確認する。
	want := []string{
		"invalid_json", "unsupported_version", "invalid_request", "invalid_state",
		"invalid_command", "invalid_coordinate", "occupied", "illegal_move",
		"not_your_turn", "stale_turn", "pass_not_allowed", "game_finished",
	}

	codes := protocol.ErrorCodes()
	if len(codes) != len(want) {
		t.Fatalf("error codeの件数が違います: got %d, want %d", len(codes), len(want))
	}
	for i, code := range codes {
		if string(code) != want[i] {
			t.Errorf("error code[%d]が違います: got %q, want %q", i, code, want[i])
		}
		if !code.Valid() {
			t.Errorf("%qがValidで拒否されました", code)
		}
	}
	if protocol.ErrorCode("nope").Valid() {
		t.Error("未知のcodeがValidで受理されました")
	}
}

func TestPlayerOpponent(t *testing.T) {
	if got := protocol.PlayerDark.Opponent(); got != protocol.PlayerLight {
		t.Errorf("darkの相手が違います: %q", got)
	}
	if got := protocol.PlayerLight.Opponent(); got != protocol.PlayerDark {
		t.Errorf("lightの相手が違います: %q", got)
	}
}

// newInitialBoard は契約の初期配置を持つ盤面を返す。
func newInitialBoard() protocol.Board {
	board := protocol.NewBoard()
	board[protocol.Index(3, 3)] = protocol.NewCell(0)
	board[protocol.Index(4, 4)] = protocol.NewCell(0)
	board[protocol.Index(3, 4)] = protocol.NewCell(255)
	board[protocol.Index(4, 3)] = protocol.NewCell(255)
	return board
}

// newPlayingState はseed=1の新規ゲームと同じ値を持つstateを返す。
func newPlayingState() protocol.State {
	return protocol.State{
		Version:           protocol.Version,
		GameID:            "unit-test",
		Board:             newInitialBoard(),
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
