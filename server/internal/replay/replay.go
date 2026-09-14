package replay

import (
	"context"
	"fmt"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

// Frame は再生中のある時点の盤面。Frames[0]は着手前の初期局面。
type Frame struct {
	// TurnNumber は適用後のturn番号。初期局面は0。
	TurnNumber int
	// MoveNumber は何手目か。1始まり。初期局面は0。
	MoveNumber int
	// Line は棋譜ソースの行番号。初期局面は0。
	Line int
	// Move はこの盤面を作った指し手。初期局面ではnil。
	Move *Move
	// State は適用後の確定state。
	State protocol.State
	// Event は適用結果のevent。初期局面ではnil。
	Event *protocol.Event
}

// Result は棋譜の再生結果。
type Result struct {
	// Script は棋譜から読み取った対局データ。
	Script Script
	// Outline は行番号と注釈の対応。
	Outline Outline
	// Frames は初期局面を先頭に、各手の盤面を順に並べたもの。
	Frames []Frame
}

// FinalState は最後の盤面を返す。
func (r Result) FinalState() protocol.State {
	return r.Frames[len(r.Frames)-1].State
}

// FrameAt は指定手数の盤面を返す。0は初期局面。
func (r Result) FrameAt(moveNumber int) (Frame, bool) {
	if moveNumber < 0 || moveNumber >= len(r.Frames) {
		return Frame{}, false
	}
	return r.Frames[moveNumber], true
}

// Record は再生結果をcontractのgame recordへまとめる。
func (r Result) Record() Record {
	commands := make([]protocol.Command, 0, len(r.Frames)-1)
	for _, frame := range r.Frames[1:] {
		if frame.Move == nil {
			continue
		}
		commands = append(commands, frame.Move.Command(frame.TurnNumber-1))
	}
	return Record{
		Version:      Version,
		RulesVersion: RulesVersion,
		GameID:       r.Script.GameID,
		Seed:         r.Script.Seed,
		StartedAt:    r.Script.StartedAt,
		EndedAt:      r.Script.EndedAt,
		Winner:       r.Script.Winner,
		Commands:     commands,
		FinalState:   r.FinalState(),
	}
}

// Replayer は棋譜を再生して盤面を復元する。
//
// 棋譜の読み取りとルールの適用は、どちらもdecoderが起動する1つのgonako
// プロセスの中で完結する。棋譜1本あたりの起動はちょうど1回で、手数に比例して
// 増えない。ルールの判断はこのpackageへ複製せず、gonako側の結果をそのまま扱う。
type Replayer struct {
	decoder *Decoder
}

// NewReplayer はreplayerを組み立てる。
func NewReplayer(decoder *Decoder) (*Replayer, error) {
	if decoder == nil {
		return nil, fmt.Errorf("decoderが必要です")
	}
	return &Replayer{decoder: decoder}, nil
}

// Options は再生の条件。
type Options struct {
	// Until は何手目まで返すか。負の値で全手。
	Until int
	// RequireFinished は終了行と終局の一致を要求する。
	RequireFinished bool
	// Strict は注釈の導出値と再生結果を突き合わせる。
	Strict bool
}

// Replay は棋譜を最後まで再生し、終局と勝者、注釈の一致まで確認する。
func (r *Replayer) Replay(ctx context.Context, source string) (*Result, error) {
	return r.ReplayWith(ctx, source, Options{Until: -1, RequireFinished: true, Strict: true})
}

// ReplayUntil は指定手数までの盤面を返す。0を渡すと初期局面だけを返す。
// 途中の盤面を取り出すためのものなので、終局と注釈の検証は行わない。
//
// 再生そのものは常に最後まで1プロセスで走るため、手数を絞っても起動回数は
// 変わらない。切り詰めるのは返すframeだけ。
func (r *Replayer) ReplayUntil(ctx context.Context, source string, moveNumber int) (*Result, error) {
	if moveNumber < 0 {
		return nil, fmt.Errorf("手数は0以上である必要があります: %d", moveNumber)
	}
	return r.ReplayWith(ctx, source, Options{Until: moveNumber})
}

// ReplayWith は条件を指定して棋譜を再生する。
func (r *Replayer) ReplayWith(ctx context.Context, source string, options Options) (*Result, error) {
	script, outline, err := r.decoder.Decode(ctx, source)
	if err != nil {
		return nil, err
	}

	if !script.OK {
		return nil, illegalMoveError(script, outline, source)
	}
	// 初期局面ぶんを足した数だけframeが返る。ここがずれるのはハーネス側の不整合。
	if len(script.Frames) != len(script.Moves)+1 {
		return nil, &SourceError{
			Code: CodeRecordSyntaxError,
			Message: fmt.Sprintf("再生結果の盤面数が指し手と合いません: 指し手%d、盤面%d",
				len(script.Moves), len(script.Frames)),
		}
	}

	limit := options.Until
	if limit < 0 || limit > len(script.Moves) {
		limit = len(script.Moves)
	}

	frames := make([]Frame, 0, limit+1)
	frames = append(frames, Frame{State: script.Frames[0].State.Clone()})

	for index := 0; index < limit; index++ {
		scriptFrame := script.Frames[index+1]
		line, _ := outline.locate(index)

		frame := Frame{
			TurnNumber: scriptFrame.TurnNumber,
			MoveNumber: scriptFrame.MoveNumber,
			Line:       line,
			Move:       &script.Moves[index],
			State:      scriptFrame.State.Clone(),
			Event:      cloneEvent(scriptFrame.Event),
		}
		frames = append(frames, frame)

		if options.Strict {
			if err := verifyAnnotation(outline, index, frame); err != nil {
				return nil, err
			}
		}
	}

	result := &Result{Script: *script, Outline: *outline, Frames: frames}

	if options.RequireFinished {
		if err := verifyFinished(result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// illegalMoveError はハーネスが適用を止めた原因を、行番号つきのerrorへ直す。
func illegalMoveError(script *Script, outline *Outline, source string) error {
	failure := script.Failure
	if failure == nil {
		return &SourceError{
			Code:    CodeRecordIllegalMove,
			Message: "棋譜を再生できません",
		}
	}

	line := 0
	if index := failure.MoveNumber - 1; index >= 0 && index < len(outline.MoveLines) {
		line = outline.MoveLines[index].Line
	}
	return &SourceError{
		Code:       CodeRecordIllegalMove,
		Line:       line,
		Source:     lineAt(source, line),
		MoveNumber: failure.MoveNumber,
		RuleCode:   failure.Code,
		Message:    failure.Message,
	}
}

// cloneEvent はeventを複製する。変更座標の配列も新しい領域を持つ。
func cloneEvent(event *protocol.Event) *protocol.Event {
	if event == nil {
		return nil
	}
	out := *event
	if event.Changes != nil {
		out.Changes = append([]protocol.Change(nil), event.Changes...)
	}
	return &out
}

// locate は指し手の行番号とソースを返す。
func (o Outline) locate(index int) (int, string) {
	if index < len(o.MoveLines) {
		return o.MoveLines[index].Line, o.MoveLines[index].Text
	}
	return 0, ""
}

// verifyAnnotation は着手行の注釈が再生結果と一致するかを確認する。
// 注釈の値は盤面から決定的に導かれるため、不一致は棋譜の破損を意味する。
func verifyAnnotation(outline *Outline, index int, frame Frame) error {
	if index >= len(outline.MoveLines) {
		return nil
	}
	moveLine := outline.MoveLines[index]
	if !moveLine.Annotated || frame.Event == nil {
		return nil
	}

	placedColor := 0
	if frame.Event.PlacedColor != nil {
		placedColor = int(*frame.Event.PlacedColor)
	}
	changed := len(frame.Event.Changes)

	if moveLine.Color != placedColor || moveLine.Changed != changed {
		return &SourceError{
			Code:       CodeRecordMismatch,
			Line:       moveLine.Line,
			Source:     moveLine.Text,
			MoveNumber: index + 1,
			Message: fmt.Sprintf("注釈と再生結果が違います (注釈=色%d・%d個変換, 再生=色%d・%d個変換)",
				moveLine.Color, moveLine.Changed, placedColor, changed),
		}
	}
	return nil
}

// verifyFinished は終了行と再生結果の終局が一致するかを確認する。
func verifyFinished(result *Result) error {
	final := result.FinalState()

	if !result.Script.Finished() {
		return &SourceError{
			Code:    CodeRecordUnfinished,
			Message: "終了行がありません",
		}
	}
	if final.Phase != protocol.PhaseFinished {
		return &SourceError{
			Code:       CodeRecordUnfinished,
			MoveNumber: len(result.Script.Moves),
			Message:    fmt.Sprintf("終了行があるのに対局が終わりません (phase=%s)", final.Phase),
		}
	}
	// どちらもnilなら引き分けどうしの一致。片方だけnilは不一致。
	if !samePlayer(final.Winner, result.Script.Winner) {
		return &SourceError{
			Code: CodeRecordMismatch,
			Message: fmt.Sprintf("記録された勝者と再生結果が違います (記録=%s, 再生=%s)",
				resultText(result.Script.Winner), resultText(final.Winner)),
		}
	}
	return nil
}

// samePlayer は勝者が一致するかを返す。どちらもnilなら引き分けどうしで一致とみなす。
func samePlayer(left, right *protocol.Player) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// resultText は勝敗の表示用文字列を返す。引き分けのnilは「引分」と書く。
func resultText(winner *protocol.Player) string {
	if winner == nil {
		return "引分"
	}
	return string(*winner)
}
