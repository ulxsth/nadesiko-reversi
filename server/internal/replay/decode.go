package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
)

// 契約の文法に対応する行のパターン。注釈は行末のコメントとして扱う。
var (
	startLinePattern = regexp.MustCompile(`^「([^」]*)」と(\d+)で対局開始\s*(?:#.*)?$`)
	placeLinePattern = regexp.MustCompile(`^([0-7])と([0-7])で(黒|白)着手\s*(?:#(.*))?$`)
	passLinePattern  = regexp.MustCompile(`^(黒|白)パス\s*(?:#.*)?$`)
	endLinePattern   = regexp.MustCompile(`^「(黒|白)」で対局終了\s*(?:#.*)?$`)
	commentPattern   = regexp.MustCompile(`^#(.*)$`)

	// 着手行の注釈。厳密照合で再生結果と突き合わせる。
	placeAnnotationPattern = regexp.MustCompile(`色(\d+)、(\d+)個変換`)
	startedAtPattern       = regexp.MustCompile(`^\s*開始\s+(\S+)`)
	endedAtPattern         = regexp.MustCompile(`^\s*終了\s+(\S+)`)
)

// outputCall は棋譜の末尾へ足す、読み取り結果の出力呼び出し。
const outputCall = "対局データ出力"

// MoveLine は棋譜中の1手と、その行番号・注釈の対応。
type MoveLine struct {
	// Number は何手目か。1始まり。
	Number int
	// Line は棋譜ソースの行番号。1始まり。
	Line int
	// Text は元の行。
	Text string
	// Annotated は着手行に色と変換数の注釈があるかどうか。
	Annotated bool
	// Color は注釈が示す駒色。範囲外の値も照合できるようintで持つ。
	Color int
	// Changed は注釈が示す変換された駒数。
	Changed int
}

// Outline は棋譜を行単位で読んだ結果。文法検査と行番号の対応に使う。
type Outline struct {
	MoveLines   []MoveLine
	StartedAt   string
	EndedAt     string
	HeaderLines int
	FooterLines int
}

// Scan は棋譜ソースを契約の文法で走査する。
//
// 実行する前に行単位で検査するので、どの行がどう文法に合わないかを
// 行番号とソースつきで返せる。
func Scan(source string) (*Outline, error) {
	outline := &Outline{}

	for index, raw := range strings.Split(source, "\n") {
		lineNumber := index + 1
		text := strings.TrimSpace(raw)
		if text == "" {
			continue
		}

		if match := commentPattern.FindStringSubmatch(text); match != nil {
			if found := startedAtPattern.FindStringSubmatch(match[1]); found != nil && outline.StartedAt == "" {
				outline.StartedAt = found[1]
			}
			if found := endedAtPattern.FindStringSubmatch(match[1]); found != nil && outline.EndedAt == "" {
				outline.EndedAt = found[1]
			}
			continue
		}

		switch {
		case startLinePattern.MatchString(text):
			outline.HeaderLines++
		case endLinePattern.MatchString(text):
			outline.FooterLines++
		case passLinePattern.MatchString(text):
			outline.MoveLines = append(outline.MoveLines, MoveLine{
				Number: len(outline.MoveLines) + 1, Line: lineNumber, Text: text,
			})
		default:
			match := placeLinePattern.FindStringSubmatch(text)
			if match == nil {
				return nil, &SourceError{
					Code:    CodeRecordSyntaxError,
					Line:    lineNumber,
					Source:  text,
					Message: "棋譜の文法に合わない行です",
				}
			}
			moveLine := MoveLine{Number: len(outline.MoveLines) + 1, Line: lineNumber, Text: text}
			if annotation := placeAnnotationPattern.FindStringSubmatch(match[4]); annotation != nil {
				color, colorErr := strconv.Atoi(annotation[1])
				changed, changedErr := strconv.Atoi(annotation[2])
				if colorErr == nil && changedErr == nil {
					// 範囲外の値もそのまま持ち、厳密照合で不一致として弾く
					moveLine.Annotated = true
					moveLine.Color = color
					moveLine.Changed = changed
				}
			}
			outline.MoveLines = append(outline.MoveLines, moveLine)
		}
	}

	return outline, nil
}

// Decoder は棋譜を再生ハーネスと組み合わせて読み取る。
type Decoder struct {
	harness string
	runner  ScriptRunner
}

// LoadHarness は再生ハーネスのソースを読む。
func LoadHarness(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("再生ハーネスを読めません: %w", err)
	}
	if !strings.Contains(string(data), "●"+outputCall+"とは") {
		return "", fmt.Errorf("再生ハーネスに%sがありません: %s", outputCall, path)
	}
	return string(data), nil
}

// NewDecoder はハーネスのソースとscript runnerからdecoderを作る。
func NewDecoder(harness string, runner ScriptRunner) (*Decoder, error) {
	if strings.TrimSpace(harness) == "" {
		return nil, fmt.Errorf("再生ハーネスのソースが必要です")
	}
	if runner == nil {
		return nil, fmt.Errorf("script runnerが必要です")
	}
	return &Decoder{harness: harness, runner: runner}, nil
}

// Decode は棋譜を実行して対局データを取り出す。
//
// 読み取りはパースではなく実行で行う。実行前に行単位の文法検査を通すので、
// 文法違反は行番号とソースつきで返る。
func (d *Decoder) Decode(ctx context.Context, source string) (*Script, *Outline, error) {
	outline, err := Scan(source)
	if err != nil {
		return nil, nil, err
	}
	if outline.HeaderLines != 1 {
		return nil, nil, &SourceError{
			Code:    CodeRecordMissingHeader,
			Message: fmt.Sprintf("開始行はちょうど1回必要です: %d回", outline.HeaderLines),
		}
	}
	if outline.FooterLines > 1 {
		return nil, nil, &SourceError{
			Code:    CodeRecordSyntaxError,
			Message: fmt.Sprintf("終了行が2回以上あります: %d回", outline.FooterLines),
		}
	}

	combined := d.harness + "\n" + source + "\n" + outputCall + "\n"
	stdout, err := d.runner.Run(ctx, combined)
	if err != nil {
		var execError *ScriptExecError
		if asScriptExecError(err, &execError) {
			line, message := d.locateFailure(execError.Stderr, source)
			return nil, nil, &SourceError{
				Code:    CodeRecordSyntaxError,
				Line:    line,
				Source:  lineAt(source, line),
				Message: message,
				Err:     err,
			}
		}
		return nil, nil, err
	}

	var script Script
	if err := json.Unmarshal(stdout, &script); err != nil {
		return nil, nil, &SourceError{
			Code:    CodeRecordSyntaxError,
			Message: "棋譜の読み取り結果を解釈できません",
			Err:     err,
		}
	}
	script.StartedAt = outline.StartedAt
	script.EndedAt = outline.EndedAt

	if script.Version != Version {
		return nil, nil, &SourceError{
			Code:    CodeRecordSyntaxError,
			Message: fmt.Sprintf("対応していない棋譜バージョンです: %q", script.Version),
		}
	}
	if err := ValidateGameID(script.GameID); err != nil {
		return nil, nil, &SourceError{
			Code:    CodeRecordMissingHeader,
			Line:    d.startLineNumber(source),
			Source:  lineAt(source, d.startLineNumber(source)),
			Message: err.Error(),
		}
	}
	for index, move := range script.Moves {
		if protocolErr := move.Validate(); protocolErr != nil {
			line := 0
			if index < len(outline.MoveLines) {
				line = outline.MoveLines[index].Line
			}
			return nil, nil, &SourceError{
				Code:       CodeRecordSyntaxError,
				Line:       line,
				Source:     lineAt(source, line),
				MoveNumber: index + 1,
				RuleCode:   protocolErr.Code,
				Message:    protocolErr.Message,
			}
		}
	}

	return &script, outline, nil
}

// gonakoErrorPattern はgonakoの診断から行番号を取り出す。
var gonakoErrorPattern = regexp.MustCompile(`\[([^\]]+)\][^(]*\((\d+)行目\)\s*[:：]\s*(.*)`)

// locateFailure はgonakoの診断を棋譜側の行番号へ直す。
// 実行したのはハーネスと棋譜を連結したソースなので、ハーネスの行数を引く。
func (d *Decoder) locateFailure(diagnostic, source string) (int, string) {
	match := gonakoErrorPattern.FindStringSubmatch(diagnostic)
	if match == nil {
		return 0, "棋譜を実行できません"
	}
	combinedLine, err := strconv.Atoi(match[2])
	if err != nil {
		return 0, "棋譜を実行できません"
	}
	message := strings.TrimSpace(match[3])
	if message == "" {
		message = strings.TrimSpace(match[1])
	}

	offset := strings.Count(d.harness, "\n") + 1
	recordLine := combinedLine - offset
	if recordLine < 1 || recordLine > strings.Count(source, "\n")+1 {
		return 0, message
	}
	return recordLine, message
}

// startLineNumber は開始行の行番号を返す。
func (d *Decoder) startLineNumber(source string) int {
	for index, raw := range strings.Split(source, "\n") {
		if startLinePattern.MatchString(strings.TrimSpace(raw)) {
			return index + 1
		}
	}
	return 0
}

// lineAt は指定行のソースを返す。
func lineAt(source string, line int) string {
	if line < 1 {
		return ""
	}
	lines := strings.Split(source, "\n")
	if line > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[line-1])
}

// 参照を保つためのcompile時チェック。
var _ = protocol.CodeInvalidJSON
