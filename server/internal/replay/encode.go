package replay

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

// 棋譜で使う手番語。契約のplayerと1対1で対応する。
const (
	darkWord  = "黒"
	lightWord = "白"
)

// gameIDPattern はgameIdの制約。ファイル名に使うため文字種を絞る。
var gameIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidateGameID はgameIdが棋譜のファイル名として使える形かを検証する。
func ValidateGameID(gameID string) error {
	if !gameIDPattern.MatchString(gameID) {
		return fmt.Errorf("対局IDは半角英数字と_-の1〜64文字です: %q", gameID)
	}
	return nil
}

// RecordFileName はgameIdに対応する棋譜のファイル名を返す。
func RecordFileName(gameID string) (string, error) {
	if err := ValidateGameID(gameID); err != nil {
		return "", err
	}
	return gameID + ".nako3", nil
}

// playerWord はplayerに対応する手番語を返す。
func playerWord(player protocol.Player) string {
	if player == protocol.PlayerLight {
		return lightWord
	}
	return darkWord
}

// wordPlayer は手番語に対応するplayerを返す。
func wordPlayer(word string) (protocol.Player, bool) {
	switch word {
	case darkWord:
		return protocol.PlayerDark, true
	case lightWord:
		return protocol.PlayerLight, true
	default:
		return "", false
	}
}

// HeaderLines は棋譜の先頭に置くヘッダ行を返す。
func HeaderLines(startedAt, endedAt string) []string {
	lines := []string{"# グラデーションリバーシ 対局記録 v1"}
	if startedAt != "" {
		lines = append(lines, "# 開始 "+startedAt)
	}
	if endedAt != "" {
		lines = append(lines, "# 終了 "+endedAt)
	}
	return lines
}

// RulesVersionLine はルール版宣言行を返す。開始行の前に1回だけ書く。
//
// 棋譜が前提とするルール版を棋譜自身へ持たせることで、ルールが変わった後でも
// 古い棋譜を黙って誤再生せず、明示的に拒否できる。
func RulesVersionLine() string {
	return fmt.Sprintf("「%s」でルール版宣言", RulesVersion)
}

// StartLine は開始行を返す。newGameの直後に1回だけ書く。
func StartLine(gameID string, seed uint32) (string, error) {
	if err := ValidateGameID(gameID); err != nil {
		return "", err
	}
	return fmt.Sprintf("「%s」と%dで対局開始", gameID, seed), nil
}

// PlaceLine は着手行を返す。
// 注釈の色と変換数は盤面から決定的に導かれる値で、人が読むためと
// 厳密照合のための自己検証を兼ねる。
func PlaceLine(player protocol.Player, row, col int, placedColor uint8, changed int) (string, error) {
	if !protocol.InBounds(row, col) {
		return "", fmt.Errorf("座標が盤外です: (%d,%d)", row, col)
	}
	if !player.Valid() {
		return "", fmt.Errorf("playerが不正です: %q", string(player))
	}
	return fmt.Sprintf("%dと%dで%s着手    # 色%d、%d個変換",
		row, col, playerWord(player), placedColor, changed), nil
}

// PassLine はパス行を返す。
func PassLine(player protocol.Player) (string, error) {
	if !player.Valid() {
		return "", fmt.Errorf("playerが不正です: %q", string(player))
	}
	return playerWord(player) + "パス", nil
}

// EndLine は終了行を返す。phaseがfinishedになった時点で1回だけ書く。
func EndLine(winner protocol.Player, moveCount, colorSum, pieceCount int) (string, error) {
	if !winner.Valid() {
		return "", fmt.Errorf("勝者が不正です: %q", string(winner))
	}
	return fmt.Sprintf("「%s」で対局終了   # %d手、色合計%d、駒%d",
		playerWord(winner), moveCount, colorSum, pieceCount), nil
}

// boardTotals は盤面の色合計と駒数を返す。終了行の注釈に使う。
func boardTotals(board protocol.Board) (colorSum, pieceCount int) {
	for _, cell := range board {
		if cell == nil {
			continue
		}
		colorSum += int(*cell)
		pieceCount++
	}
	return colorSum, pieceCount
}

// EncodeResult は再生結果を契約の棋譜へ直列化する。
//
// 注釈には各手の駒色と変換数、終了行には総手数と盤面の集計を書く。
// どれも盤面から決定的に導かれる値なので、再実行すれば検証できる。
func EncodeResult(result *Result) (string, error) {
	if result == nil || len(result.Frames) == 0 {
		return "", fmt.Errorf("再生結果がありません")
	}

	script := result.Script
	if err := ValidateGameID(script.GameID); err != nil {
		return "", err
	}

	var out strings.Builder
	for _, line := range HeaderLines(script.StartedAt, script.EndedAt) {
		out.WriteString(line)
		out.WriteString("\n")
	}
	out.WriteString("\n")

	out.WriteString(RulesVersionLine())
	out.WriteString("\n")

	startLine, err := StartLine(script.GameID, script.Seed)
	if err != nil {
		return "", err
	}
	out.WriteString(startLine)
	out.WriteString("\n\n")

	for _, frame := range result.Frames[1:] {
		if frame.Move == nil {
			continue
		}
		line, err := encodeFrame(frame)
		if err != nil {
			return "", fmt.Errorf("%d手目を書き出せません: %w", frame.MoveNumber, err)
		}
		out.WriteString(line)
		out.WriteString("\n")
	}

	final := result.FinalState()
	if final.Phase != protocol.PhaseFinished {
		// 終了行のない記録は進行中とみなす。ここまでを棋譜として返す。
		return out.String(), nil
	}
	if final.Winner == nil {
		return "", fmt.Errorf("終局しているのに勝者がありません")
	}

	colorSum, pieceCount := boardTotals(final.Board)
	endLine, err := EndLine(*final.Winner, len(result.Frames)-1, colorSum, pieceCount)
	if err != nil {
		return "", err
	}
	out.WriteString("\n")
	out.WriteString(endLine)
	out.WriteString("\n")

	return out.String(), nil
}

// encodeFrame は1手分の行を作る。
func encodeFrame(frame Frame) (string, error) {
	move := frame.Move
	switch move.Type {
	case protocol.CommandPass:
		return PassLine(move.Player)
	case protocol.CommandPlace:
		if move.Row == nil || move.Col == nil {
			return "", fmt.Errorf("着手に座標がありません")
		}
		var placedColor uint8
		changed := 0
		if frame.Event != nil {
			if frame.Event.PlacedColor != nil {
				placedColor = *frame.Event.PlacedColor
			}
			changed = len(frame.Event.Changes)
		}
		return PlaceLine(move.Player, *move.Row, *move.Col, placedColor, changed)
	default:
		return "", fmt.Errorf("未対応の指し手です: %q", string(move.Type))
	}
}
