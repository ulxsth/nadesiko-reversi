// Package protocol は docs/contracts のJSON契約v1をGoの型として表現する。
// このpackageは搬送手段を持たず、encode/decodeと妥当性検証だけを担う。
package protocol

import "encoding/json"

// Version は契約が要求するenvelope version。
const Version = "1"

// 盤面の寸法。
const (
	BoardSize  = 8
	BoardCells = BoardSize * BoardSize
)

// Player は手番の所属を表す。駒の色とは独立している。
type Player string

const (
	PlayerDark  Player = "dark"
	PlayerLight Player = "light"
)

// Valid はplayerが契約の列挙値かどうかを返す。
func (p Player) Valid() bool {
	return p == PlayerDark || p == PlayerLight
}

// Opponent は相手のplayerを返す。
func (p Player) Opponent() Player {
	if p == PlayerDark {
		return PlayerLight
	}
	return PlayerDark
}

// Phase は対局の進行状態を表す。
type Phase string

const (
	PhasePlaying  Phase = "playing"
	PhaseFinished Phase = "finished"
)

// Valid はphaseが契約の列挙値かどうかを返す。
func (p Phase) Valid() bool {
	return p == PhasePlaying || p == PhaseFinished
}

// Action はruntimeへの要求種別を表す。
type Action string

const (
	ActionNewGame      Action = "newGame"
	ActionApplyCommand Action = "applyCommand"
)

// CommandType はplayerの操作種別を表す。
type CommandType string

const (
	CommandPlace CommandType = "place"
	CommandPass  CommandType = "pass"
)

// EventType は受理されたcommandに対応する結果種別を表す。
type EventType string

const (
	EventPlaced EventType = "placed"
	EventPassed EventType = "passed"
)

// Cell は1マスの内容を表す。nilは空きマス、それ以外は0〜255の駒色。
type Cell = *uint8

// NewCell は指定色の駒を持つマスを返す。
func NewCell(color uint8) Cell {
	return &color
}

// Board はrow-majorな64マスの盤面。indexはrow*8+col。
type Board []Cell

// NewBoard は全マスが空の盤面を返す。
func NewBoard() Board {
	return make(Board, BoardCells)
}

// Index はrow、colに対応するboardのindexを返す。
func Index(row, col int) int {
	return row*BoardSize + col
}

// At はrow、colのマスを返す。盤外の場合は空きマスとして扱う。
func (b Board) At(row, col int) Cell {
	if !InBounds(row, col) || len(b) != BoardCells {
		return nil
	}
	return b[Index(row, col)]
}

// Clone は盤面と各マスの値を複製する。呼び出し側の変更が元へ伝播しない。
func (b Board) Clone() Board {
	if b == nil {
		return nil
	}
	out := make(Board, len(b))
	for i, cell := range b {
		if cell != nil {
			out[i] = NewCell(*cell)
		}
	}
	return out
}

// InBounds はrow、colが盤内かどうかを返す。
func InBounds(row, col int) bool {
	return row >= 0 && row < BoardSize && col >= 0 && col < BoardSize
}

// Position は盤上の座標を表す。
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Command はplayerが送る1件の操作。passはRow、Colを持たない。
type Command struct {
	CommandID    string      `json:"commandId"`
	Type         CommandType `json:"type"`
	Player       Player      `json:"player"`
	ExpectedTurn int         `json:"expectedTurn"`
	Row          *int        `json:"row,omitempty"`
	Col          *int        `json:"col,omitempty"`
}

// PlaceCommand は着手commandを組み立てる。
func PlaceCommand(commandID string, player Player, expectedTurn, row, col int) Command {
	return Command{
		CommandID:    commandID,
		Type:         CommandPlace,
		Player:       player,
		ExpectedTurn: expectedTurn,
		Row:          &row,
		Col:          &col,
	}
}

// PassCommand はパスcommandを組み立てる。
func PassCommand(commandID string, player Player, expectedTurn int) Command {
	return Command{
		CommandID:    commandID,
		Type:         CommandPass,
		Player:       player,
		ExpectedTurn: expectedTurn,
	}
}

// State は対局の完全な状態。runtimeはこの値だけを入力として評価する。
type State struct {
	Version           string     `json:"version"`
	GameID            string     `json:"gameId"`
	Board             Board      `json:"board"`
	TurnNumber        int        `json:"turnNumber"`
	CurrentPlayer     Player     `json:"currentPlayer"`
	NextColor         uint8      `json:"nextColor"`
	RNGState          uint32     `json:"rngState"`
	ConsecutivePasses int        `json:"consecutivePasses"`
	Phase             Phase      `json:"phase"`
	Winner            *Player    `json:"winner"`
	LegalMoves        []Position `json:"legalMoves"`
}

// MarshalJSON はlegalMovesがnilのときもJSON arrayとして出力する。
// contractではlegalMovesが必須のarrayなので、nullを出さない。
func (s State) MarshalJSON() ([]byte, error) {
	type stateAlias State
	clone := stateAlias(s)
	if clone.LegalMoves == nil {
		clone.LegalMoves = []Position{}
	}
	return json.Marshal(clone)
}

// Clone はstateを複製する。盤面とlegalMovesも新しい領域を持つ。
func (s State) Clone() State {
	out := s
	out.Board = s.Board.Clone()
	if s.LegalMoves != nil {
		out.LegalMoves = append([]Position(nil), s.LegalMoves...)
	}
	if s.Winner != nil {
		winner := *s.Winner
		out.Winner = &winner
	}
	return out
}

// IsLegalMove はrow、colが現在の合法手に含まれるかどうかを返す。
func (s State) IsLegalMove(row, col int) bool {
	for _, move := range s.LegalMoves {
		if move.Row == row && move.Col == col {
			return true
		}
	}
	return false
}

// Change は着手によって色が変わった1駒を表す。
type Change struct {
	Row   int   `json:"row"`
	Col   int   `json:"col"`
	Color uint8 `json:"color"`
}

// Event は受理されたcommandの結果。着手のときだけ座標と変換情報を持つ。
type Event struct {
	CommandID   string    `json:"commandId"`
	Type        EventType `json:"type"`
	Player      Player    `json:"player"`
	TurnNumber  int       `json:"turnNumber"`
	Row         *int      `json:"row,omitempty"`
	Col         *int      `json:"col,omitempty"`
	PlacedColor *uint8    `json:"placedColor,omitempty"`
	Changes     []Change  `json:"changes,omitempty"`
}

// Request はruntimeへの1回の要求。actionによって必要なfieldが変わる。
type Request struct {
	Version string   `json:"version"`
	Action  Action   `json:"action"`
	GameID  string   `json:"gameId,omitempty"`
	Seed    *uint32  `json:"seed,omitempty"`
	State   *State   `json:"state,omitempty"`
	Command *Command `json:"command,omitempty"`
}

// NewGameRequest は新規ゲーム要求を組み立てる。
func NewGameRequest(gameID string, seed uint32) Request {
	return Request{
		Version: Version,
		Action:  ActionNewGame,
		GameID:  gameID,
		Seed:    &seed,
	}
}

// ApplyCommandRequest はcommand適用要求を組み立てる。
func ApplyCommandRequest(state State, command Command) Request {
	return Request{
		Version: Version,
		Action:  ActionApplyCommand,
		State:   &state,
		Command: &command,
	}
}

// Response はruntimeからの1回の応答。成功時はState、拒否時はErrorを持つ。
type Response struct {
	Version string `json:"version"`
	OK      bool   `json:"ok"`
	State   *State `json:"state,omitempty"`
	Event   *Event `json:"event,omitempty"`
	Error   *Error `json:"error,omitempty"`
}
