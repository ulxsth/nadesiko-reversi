package replay

import (
	"context"
	"fmt"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
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
		Version:    Version,
		GameID:     r.Script.GameID,
		Seed:       r.Script.Seed,
		StartedAt:  r.Script.StartedAt,
		EndedAt:    r.Script.EndedAt,
		Winner:     r.Script.Winner,
		Commands:   commands,
		FinalState: r.FinalState(),
	}
}

// Replayer は棋譜を再生して盤面を復元する。
//
// ルールの適用は#3のruntime.Runnerだけを通すので、ルールの判断をこのpackageへ
// 複製しない。契約は再生ハーネスがrules/game/main.nako3へ委譲する形を想定するが、
// main.nako3が末尾で無条件にCLI実行するため取り込めない。Issue #6のブロッカーと
// して記録済みで、取り込みガードが入るまでは同じルールengineへGo側から再適用する。
type Replayer struct {
	decoder    *Decoder
	ruleRunner runtime.Runner
}

// NewReplayer はreplayerを組み立てる。
func NewReplayer(decoder *Decoder, ruleRunner runtime.Runner) (*Replayer, error) {
	if decoder == nil {
		return nil, fmt.Errorf("decoderが必要です")
	}
	if ruleRunner == nil {
		return nil, fmt.Errorf("ruleRunnerが必要です")
	}
	return &Replayer{decoder: decoder, ruleRunner: ruleRunner}, nil
}

// Options は再生の条件。
type Options struct {
	// Until は何手目まで再生するか。負の値で全手。
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

// ReplayUntil は指定手数まで再生する。0を渡すと初期局面だけを返す。
// 途中の盤面を取り出すためのものなので、終局と注釈の検証は行わない。
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

	limit := options.Until
	if limit < 0 || limit > len(script.Moves) {
		limit = len(script.Moves)
	}

	response, err := runtime.NewGame(ctx, r.ruleRunner, script.GameID, script.Seed)
	if err != nil {
		return nil, err
	}
	if !response.OK || response.State == nil {
		code, message := responseFailure(response, "新しい対局を開始できません")
		return nil, &SourceError{Code: CodeRecordIllegalMove, RuleCode: code, Message: message}
	}

	frames := make([]Frame, 0, limit+1)
	frames = append(frames, Frame{State: response.State.Clone()})
	current := response.State.Clone()

	for index := 0; index < limit; index++ {
		move := script.Moves[index]
		line, text := outline.locate(index)

		response, err := runtime.ApplyCommand(ctx, r.ruleRunner, current, move.Command(current.TurnNumber))
		if err != nil {
			return nil, err
		}
		if !response.OK || response.State == nil {
			code, message := responseFailure(response, "この手を再生できません")
			return nil, &SourceError{
				Code:       CodeRecordIllegalMove,
				Line:       line,
				Source:     text,
				MoveNumber: index + 1,
				RuleCode:   code,
				Message:    message,
			}
		}

		current = response.State.Clone()
		frame := Frame{
			TurnNumber: current.TurnNumber,
			MoveNumber: index + 1,
			Line:       line,
			Move:       &script.Moves[index],
			State:      current.Clone(),
		}
		if response.Event != nil {
			event := *response.Event
			if response.Event.Changes != nil {
				event.Changes = append([]protocol.Change(nil), response.Event.Changes...)
			}
			frame.Event = &event
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
	if final.Winner == nil || *final.Winner != result.Script.Winner {
		replayed := "なし"
		if final.Winner != nil {
			replayed = string(*final.Winner)
		}
		return &SourceError{
			Code: CodeRecordMismatch,
			Message: fmt.Sprintf("記録された勝者と再生結果が違います (記録=%s, 再生=%s)",
				result.Script.Winner, replayed),
		}
	}
	return nil
}

// responseFailure は拒否responseから契約のcodeとmessageを取り出す。
func responseFailure(response *protocol.Response, fallback string) (protocol.ErrorCode, string) {
	if response != nil && response.Error != nil {
		return response.Error.Code, response.Error.Message
	}
	return protocol.CodeInvalidState, fallback
}
