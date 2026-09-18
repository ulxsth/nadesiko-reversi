// Package replay は確定した対局を実行可能ななでしこコードとして保存し、
// 再実行によって各手の盤面を復元する。
//
// 棋譜はdocs/contracts/game-record.schema.jsonのrecordと1対1で対応するが、
// 保存形式はJSONではなく、そのままgonakoで実行できる日本語コードにする。
package replay

import (
	"fmt"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

// Version は棋譜形式のversion。contractのenvelope versionと揃える。
const Version = protocol.Version

// RulesVersion はこのサーバーが再生できるルール版。棋譜の宣言行と突き合わせる。
// ルールの振る舞いが変わったらここを上げ、古い棋譜は明示的に拒否する。
//
// 版2で色変換を四捨五入へ変え、色の重心がちょうど中央の終局を引き分けにした。
// 版1の棋譜は同じseedでも別の盤面になるため再生できない。
const RulesVersion = "2"

// Move は棋譜1手分の指し手。
// contractのcommandからcommandIdとexpectedTurnを除いたもので、
// この2つは再生時のturn順から一意に決まるため棋譜には書かない。
type Move struct {
	Player protocol.Player      `json:"player"`
	Type   protocol.CommandType `json:"type"`
	Row    *int                 `json:"row,omitempty"`
	Col    *int                 `json:"col,omitempty"`
}

// PlaceMove は着手を組み立てる。
func PlaceMove(player protocol.Player, row, col int) Move {
	return Move{Player: player, Type: protocol.CommandPlace, Row: &row, Col: &col}
}

// PassMove はパスを組み立てる。
func PassMove(player protocol.Player) Move {
	return Move{Player: player, Type: protocol.CommandPass}
}

// Validate はmoveが棋譜として成立する形かどうかを検証する。
func (m Move) Validate() *protocol.Error {
	if !m.Player.Valid() {
		return protocol.NewError(protocol.CodeInvalidCommand, "playerが不正です")
	}
	switch m.Type {
	case protocol.CommandPlace:
		if m.Row == nil || m.Col == nil {
			return protocol.NewError(protocol.CodeInvalidCommand, "着手にはrowとcolが必要です")
		}
		if !protocol.InBounds(*m.Row, *m.Col) {
			return protocol.NewError(protocol.CodeInvalidCoordinate, "座標が盤外です")
		}
		return nil
	case protocol.CommandPass:
		if m.Row != nil || m.Col != nil {
			return protocol.NewError(protocol.CodeInvalidCommand, "パスはrowとcolを持てません")
		}
		return nil
	default:
		return protocol.NewError(protocol.CodeInvalidCommand, fmt.Sprintf("未対応の指し手です: %q", string(m.Type)))
	}
}

// Command はturn番号を与えてcontractのcommandへ変換する。
// commandIdは再生の再現性を保つため、turn番号から決定的に作る。
func (m Move) Command(turnNumber int) protocol.Command {
	command := protocol.Command{
		CommandID:    fmt.Sprintf("replay-%d", turnNumber+1),
		Type:         m.Type,
		Player:       m.Player,
		ExpectedTurn: turnNumber,
	}
	if m.Type == protocol.CommandPlace && m.Row != nil && m.Col != nil {
		row, col := *m.Row, *m.Col
		command.Row = &row
		command.Col = &col
	}
	return command
}

// MoveFromCommand はcontractのcommandから棋譜1手分を取り出す。
func MoveFromCommand(command protocol.Command) Move {
	move := Move{Player: command.Player, Type: command.Type}
	if command.Type == protocol.CommandPlace && command.Row != nil && command.Col != nil {
		row, col := *command.Row, *command.Col
		move.Row = &row
		move.Col = &col
	}
	return move
}

// Script は棋譜を実行して読み取った対局データ。
// 盤面は持たず、再生に必要な入力だけを保持する。
type Script struct {
	Version string `json:"version"`
	GameID  string `json:"gameId"`
	Seed    uint32 `json:"seed"`
	// Winner は終了行の手番語。引き分けの終了行とnilで対応する。
	// 終了行が無い進行中の記録でもnilになるため、区別はFinishedで行う。
	Winner *protocol.Player `json:"winner"`
	Moves  []Move           `json:"moves"`
	// HeaderCount は開始行の出現回数。1以外はrecord_missing_header。
	HeaderCount int `json:"headerCount"`
	// FooterCount は終了行の出現回数。0は進行中の記録。
	FooterCount int `json:"footerCount"`
	// RulesVersion は版宣言行が示すルール版。
	RulesVersion string `json:"rulesVersion"`
	// VersionCount は版宣言行の出現回数。1以外はrecord_missing_header。
	VersionCount int `json:"versionCount"`
	// OK はハーネスが全手を適用できたかどうか。
	OK bool `json:"ok"`
	// Failure は適用が止まった原因。OKが真のときはnil。
	Failure *ScriptFailure `json:"error,omitempty"`
	// Frames は初期局面を先頭に、各手の適用後stateを順に並べたもの。
	Frames []ScriptFrame `json:"frames"`
	// StartedAt と EndedAt はヘッダ注釈から読む。棋譜本体には現れない。
	StartedAt string `json:"-"`
	EndedAt   string `json:"-"`
}

// Finished は終了行を持つ完了記録かどうかを返す。
func (s Script) Finished() bool {
	return s.FooterCount == 1
}

// ScriptFrame はハーネスが返す、ある時点の確定state。
type ScriptFrame struct {
	TurnNumber int             `json:"turnNumber"`
	MoveNumber int             `json:"moveNumber"`
	State      protocol.State  `json:"state"`
	Event      *protocol.Event `json:"event,omitempty"`
}

// ScriptFailure はハーネスが適用を止めた原因。ルール側のcodeをそのまま持つ。
type ScriptFailure struct {
	MoveNumber int                `json:"moveNumber"`
	Code       protocol.ErrorCode `json:"code"`
	Message    string             `json:"message"`
}

// Record はcontractのgame record v1。再生で確定した最終stateを含む。
type Record struct {
	Version      string `json:"version"`
	RulesVersion string `json:"rulesVersion"`
	GameID       string `json:"gameId"`
	Seed         uint32 `json:"seed"`
	StartedAt    string `json:"startedAt"`
	EndedAt      string `json:"endedAt"`
	// Winner は最終結果。引き分けはnilで表す。
	Winner     *protocol.Player   `json:"winner"`
	Commands   []protocol.Command `json:"commands"`
	FinalState protocol.State     `json:"finalState"`
}

// Script はrecordから棋譜ソース用の対局データを取り出す。
func (r Record) Script() Script {
	moves := make([]Move, 0, len(r.Commands))
	for _, command := range r.Commands {
		moves = append(moves, MoveFromCommand(command))
	}
	return Script{
		Version:      Version,
		RulesVersion: RulesVersion,
		GameID:       r.GameID,
		Seed:         r.Seed,
		StartedAt:    r.StartedAt,
		EndedAt:      r.EndedAt,
		Winner:       clonePlayer(r.Winner),
		Moves:        moves,
	}
}

// clonePlayer は勝者のポインタを複製する。引き分けのnilはnilのまま返す。
func clonePlayer(player *protocol.Player) *protocol.Player {
	if player == nil {
		return nil
	}
	out := *player
	return &out
}

// Summary は一覧表示用の要約。
type Summary struct {
	GameID string `json:"gameId"`
	Seed   uint32 `json:"seed"`
	// Winner は最終結果。引き分けはnilで表す。
	Winner    *protocol.Player `json:"winner"`
	MoveCount int              `json:"moveCount"`
	StartedAt string           `json:"startedAt"`
	EndedAt   string           `json:"endedAt"`
}

// Summary はrecordの要約を返す。
func (r Record) Summary() Summary {
	return Summary{
		GameID:    r.GameID,
		Seed:      r.Seed,
		Winner:    clonePlayer(r.Winner),
		MoveCount: len(r.Commands),
		StartedAt: r.StartedAt,
		EndedAt:   r.EndedAt,
	}
}

// Clone はrecordを複製する。盤面とcommand列も新しい領域を持つ。
func (r Record) Clone() Record {
	out := r
	out.Winner = clonePlayer(r.Winner)
	out.Commands = append([]protocol.Command(nil), r.Commands...)
	for i, command := range r.Commands {
		if command.Row != nil {
			row := *command.Row
			out.Commands[i].Row = &row
		}
		if command.Col != nil {
			col := *command.Col
			out.Commands[i].Col = &col
		}
	}
	out.FinalState = r.FinalState.Clone()
	return out
}
