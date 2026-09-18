package replay_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/replay"
)

// fakeScriptRunner は棋譜を実行せず、決めた出力を返す。
type fakeScriptRunner struct {
	output  []byte
	err     error
	lastRun string
}

func (f *fakeScriptRunner) Run(_ context.Context, source string) ([]byte, error) {
	f.lastRun = source
	if f.err != nil {
		return nil, f.err
	}
	return f.output, nil
}

// newFakeDecoder は実行結果を固定したdecoderを作る。
func newFakeDecoder(t *testing.T, output string) (*replay.Decoder, *fakeScriptRunner) {
	t.Helper()
	runner := &fakeScriptRunner{output: []byte(output)}
	decoder, err := replay.NewDecoder("●対局データ出力とは\nここまで\n", runner)
	if err != nil {
		t.Fatalf("decoderを作れません: %v", err)
	}
	return decoder, runner
}

func TestScanReportsSyntaxErrorWithLine(t *testing.T) {
	source := strings.Join([]string{
		"# 記録",
		"",
		"「demo-1」と1で対局開始",
		"2と2で黒着手",
		"ここは文法に合わない",
		"「白」で対局終了",
	}, "\n")

	_, err := replay.Scan(source)
	if err == nil {
		t.Fatal("文法違反が検出されません")
	}

	var sourceErr *replay.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("SourceErrorではありません: %T %v", err, err)
	}
	if sourceErr.Code != replay.CodeRecordSyntaxError {
		t.Errorf("codeが違います: %q", sourceErr.Code)
	}
	if sourceErr.Line != 5 {
		t.Errorf("行番号が違います: %d", sourceErr.Line)
	}
	if sourceErr.Source != "ここは文法に合わない" {
		t.Errorf("該当行のソースが違います: %q", sourceErr.Source)
	}
}

func TestScanRejectsOutOfRangeCoordinate(t *testing.T) {
	// 座標は0〜7。8は文法段階で弾く。
	source := "「demo-1」と1で対局開始\n8と2で黒着手\n"

	_, err := replay.Scan(source)
	var sourceErr *replay.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("SourceErrorではありません: %T %v", err, err)
	}
	if sourceErr.Line != 2 {
		t.Errorf("行番号が違います: %d", sourceErr.Line)
	}
}

func TestScanCollectsAnnotationsAndTimes(t *testing.T) {
	source := strings.Join([]string{
		"# グラデーションリバーシ 対局記録 v1",
		"# 開始 2026-09-12T21:30:00+09:00",
		"# 終了 2026-09-12T21:44:12+09:00",
		"",
		"「demo-1」と1で対局開始",
		"2と2で黒着手    # 色60、3個変換",
		"白パス",
		"「白」で対局終了   # 61手、色合計8891、駒64",
	}, "\n")

	outline, err := replay.Scan(source)
	if err != nil {
		t.Fatalf("走査に失敗: %v", err)
	}
	if outline.StartedAt != "2026-09-12T21:30:00+09:00" {
		t.Errorf("開始時刻が違います: %q", outline.StartedAt)
	}
	if outline.EndedAt != "2026-09-12T21:44:12+09:00" {
		t.Errorf("終了時刻が違います: %q", outline.EndedAt)
	}
	if outline.HeaderLines != 1 || outline.FooterLines != 1 {
		t.Errorf("開始行・終了行の数が違います: %d / %d", outline.HeaderLines, outline.FooterLines)
	}
	if len(outline.MoveLines) != 2 {
		t.Fatalf("手数が違います: %d", len(outline.MoveLines))
	}
	first := outline.MoveLines[0]
	if !first.Annotated || first.Color != 60 || first.Changed != 3 || first.Line != 6 {
		t.Errorf("着手行の注釈が違います: %+v", first)
	}
	if outline.MoveLines[1].Annotated {
		t.Error("パス行に注釈が付いています")
	}
}

func TestDecodeRejectsMissingOrDuplicatedHeader(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "開始行がない", source: "「2」でルール版宣言\n2と2で黒着手\n"},
		{
			name:   "開始行が2回ある",
			source: "「2」でルール版宣言\n「demo-1」と1で対局開始\n「demo-2」と2で対局開始\n2と2で黒着手\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder, _ := newFakeDecoder(t, "{}")
			_, _, err := decoder.Decode(context.Background(), tt.source)
			var sourceErr *replay.SourceError
			if !errors.As(err, &sourceErr) {
				t.Fatalf("SourceErrorではありません: %T %v", err, err)
			}
			if sourceErr.Code != replay.CodeRecordMissingHeader {
				t.Errorf("codeが違います: %q", sourceErr.Code)
			}
		})
	}
}

func TestDecodeAppendsOutputCallToHarness(t *testing.T) {
	source := "「2」でルール版宣言\n「demo-1」と1で対局開始\n2と2で黒着手\n"
	decoder, runner := newFakeDecoder(t,
		`{"version":"1","rulesVersion":"2","ok":true,"gameId":"demo-1","seed":1,"winner":null,"moves":[{"player":"dark","type":"place","row":2,"col":2}],"headerCount":1,"footerCount":0}`)

	script, outline, err := decoder.Decode(context.Background(), source)
	if err != nil {
		t.Fatalf("読み取りに失敗: %v", err)
	}

	// ハーネス + 棋譜 + 出力呼び出し の順に連結されていること
	if !strings.HasPrefix(runner.lastRun, "●対局データ出力とは") {
		t.Error("ハーネスが先頭にありません")
	}
	if !strings.Contains(runner.lastRun, "2と2で黒着手") {
		t.Error("棋譜が連結されていません")
	}
	if !strings.HasSuffix(strings.TrimSpace(runner.lastRun), "対局データ出力") {
		t.Error("出力呼び出しが末尾にありません")
	}

	if script.Finished() {
		t.Error("終了行がないのに完了扱いです")
	}
	if len(outline.MoveLines) != 1 {
		t.Errorf("行の対応が違います: %d", len(outline.MoveLines))
	}
}

func TestDecodeRejectsInvalidGameID(t *testing.T) {
	source := "「2」でルール版宣言\n「demo 1」と1で対局開始\n2と2で黒着手\n"
	decoder, _ := newFakeDecoder(t,
		`{"version":"1","rulesVersion":"2","ok":true,"gameId":"demo 1","seed":1,"winner":null,"moves":[{"player":"dark","type":"place","row":2,"col":2}],"headerCount":1,"footerCount":0}`)

	_, _, err := decoder.Decode(context.Background(), source)
	var sourceErr *replay.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("SourceErrorではありません: %T %v", err, err)
	}
	if sourceErr.Code != replay.CodeRecordMissingHeader {
		t.Errorf("codeが違います: %q (%s)", sourceErr.Code, sourceErr.Message)
	}
	if sourceErr.Line != 2 {
		t.Errorf("行番号が違います: %d", sourceErr.Line)
	}
}

func TestDecodeMapsGonakoDiagnosticToRecordLine(t *testing.T) {
	source := "「2」でルール版宣言\n「demo-1」と1で対局開始\n2と2で黒着手\n白パス\n"
	// ハーネス2行 + 区切りの空行1行。連結後の6行目 = 棋譜の3行目。
	runner := &fakeScriptRunner{err: &replay.ScriptExecError{
		ExitCode: 1,
		Stderr:   "[文法エラー]/tmp/x/record.nako3(6行目): 不完全な文です。",
	}}
	decoder, err := replay.NewDecoder("●対局データ出力とは\nここまで\n", runner)
	if err != nil {
		t.Fatalf("decoderを作れません: %v", err)
	}

	_, _, err = decoder.Decode(context.Background(), source)
	var sourceErr *replay.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("SourceErrorではありません: %T %v", err, err)
	}
	if sourceErr.Code != replay.CodeRecordSyntaxError {
		t.Errorf("codeが違います: %q", sourceErr.Code)
	}
	if sourceErr.Line != 3 {
		t.Errorf("棋譜側の行番号へ直せていません: %d", sourceErr.Line)
	}
	if sourceErr.Source != "2と2で黒着手" {
		t.Errorf("該当行のソースが違います: %q", sourceErr.Source)
	}
}

func TestRecordErrorCodesCoverContract(t *testing.T) {
	want := []string{
		"record_syntax_error", "record_missing_header", "record_illegal_move",
		"record_mismatch", "record_unfinished", "record_unsupported_rules_version",
	}
	codes := replay.RecordErrorCodes()
	if len(codes) != len(want) {
		t.Fatalf("codeの件数が違います: got %d, want %d", len(codes), len(want))
	}
	for i, code := range codes {
		if string(code) != want[i] {
			t.Errorf("code[%d]が違います: got %q, want %q", i, code, want[i])
		}
		if !code.Valid() {
			t.Errorf("%qがValidで拒否されました", code)
		}
	}
	if replay.RecordErrorCode("nope").Valid() {
		t.Error("未知のcodeがValidで受理されました")
	}
}

func TestEncodeLines(t *testing.T) {
	start, err := replay.StartLine("demo-1", 1)
	if err != nil {
		t.Fatalf("開始行を作れません: %v", err)
	}
	if start != "「demo-1」と1で対局開始" {
		t.Errorf("開始行が違います: %q", start)
	}

	place, err := replay.PlaceLine(protocol.PlayerDark, 2, 2, 60, 3)
	if err != nil {
		t.Fatalf("着手行を作れません: %v", err)
	}
	if !strings.HasPrefix(place, "2と2で黒着手") || !strings.Contains(place, "# 色60、3個変換") {
		t.Errorf("着手行が違います: %q", place)
	}

	pass, err := replay.PassLine(protocol.PlayerLight)
	if err != nil {
		t.Fatalf("パス行を作れません: %v", err)
	}
	if pass != "白パス" {
		t.Errorf("パス行が違います: %q", pass)
	}

	light := protocol.PlayerLight
	end, err := replay.EndLine(&light, 61, 8891, 64)
	if err != nil {
		t.Fatalf("終了行を作れません: %v", err)
	}
	if !strings.HasPrefix(end, "「白」で対局終了") || !strings.Contains(end, "# 61手、色合計8891、駒64") {
		t.Errorf("終了行が違います: %q", end)
	}

	// 勝者のいない終局は引分の終了行になる
	draw, err := replay.EndLine(nil, 60, 8160, 64)
	if err != nil {
		t.Fatalf("引分の終了行を作れません: %v", err)
	}
	if !strings.HasPrefix(draw, "「引分」で対局終了") || !strings.Contains(draw, "# 60手、色合計8160、駒64") {
		t.Errorf("引分の終了行が違います: %q", draw)
	}

	// 生成した行をそのまま走査で読み直せること
	source := strings.Join([]string{start, place, pass, end}, "\n")
	outline, err := replay.Scan(source)
	if err != nil {
		t.Fatalf("生成した棋譜を走査できません: %v", err)
	}
	if len(outline.MoveLines) != 2 || outline.HeaderLines != 1 || outline.FooterLines != 1 {
		t.Errorf("走査結果が違います: %+v", outline)
	}

	// 引分の終了行も終了行として走査できる
	drawOutline, err := replay.Scan(strings.Join([]string{start, place, draw}, "\n"))
	if err != nil {
		t.Fatalf("引分の棋譜を走査できません: %v", err)
	}
	if drawOutline.FooterLines != 1 {
		t.Errorf("引分の終了行を数えられません: %+v", drawOutline)
	}
}

func TestValidateGameIDAndFileName(t *testing.T) {
	for _, valid := range []string{"demo-1", "a", "A_b-9", strings.Repeat("x", 64)} {
		if err := replay.ValidateGameID(valid); err != nil {
			t.Errorf("%qが拒否されました: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "demo 1", "デモ", "a/b", strings.Repeat("x", 65)} {
		if err := replay.ValidateGameID(invalid); err == nil {
			t.Errorf("%qが受理されました", invalid)
		}
	}

	name, err := replay.RecordFileName("demo-1")
	if err != nil {
		t.Fatalf("ファイル名を作れません: %v", err)
	}
	if name != "demo-1.nako3" {
		t.Errorf("ファイル名が違います: %q", name)
	}
}

// drawScriptOutput は引き分けで終わった棋譜のハーネス出力を組み立てる。
//
// (0,0)=0と(7,7)=255だけが置かれた盤面はどの空きマスからも駒を挟めないので
// 合法手が0件になり、色合計255・駒2で`2*255 == 2*255`が成り立つ。
// 両者がパスすると勝者のいない終局になる。
func drawScriptOutput(t *testing.T) string {
	t.Helper()

	board := protocol.NewBoard()
	board[0] = protocol.NewCell(0)
	board[63] = protocol.NewCell(255)

	start := protocol.State{
		Version:       protocol.Version,
		GameID:        "draw-1",
		Board:         board,
		TurnNumber:    0,
		CurrentPlayer: protocol.PlayerDark,
		NextColor:     7,
		RNGState:      1234,
		Phase:         protocol.PhasePlaying,
		LegalMoves:    []protocol.Position{},
	}

	afterFirstPass := start.Clone()
	afterFirstPass.TurnNumber = 1
	afterFirstPass.CurrentPlayer = protocol.PlayerLight
	afterFirstPass.ConsecutivePasses = 1

	finished := afterFirstPass.Clone()
	finished.TurnNumber = 2
	finished.CurrentPlayer = protocol.PlayerDark
	finished.ConsecutivePasses = 2
	finished.Phase = protocol.PhaseFinished
	finished.Winner = nil
	finished.LegalMoves = []protocol.Position{}

	script := replay.Script{
		Version:      replay.Version,
		RulesVersion: replay.RulesVersion,
		GameID:       "draw-1",
		Seed:         1,
		Winner:       nil,
		Moves:        []replay.Move{replay.PassMove(protocol.PlayerDark), replay.PassMove(protocol.PlayerLight)},
		HeaderCount:  1,
		FooterCount:  1,
		VersionCount: 1,
		OK:           true,
		Frames: []replay.ScriptFrame{
			{TurnNumber: 0, MoveNumber: 0, State: start},
			{TurnNumber: 1, MoveNumber: 1, State: afterFirstPass},
			{TurnNumber: 2, MoveNumber: 2, State: finished},
		},
	}

	encoded, err := json.Marshal(script)
	if err != nil {
		t.Fatalf("棋譜の出力を組み立てられません: %v", err)
	}
	return string(encoded)
}

// TestReplayAcceptsDrawRecord は引き分けの棋譜が不一致にならないことを確かめる。
func TestReplayAcceptsDrawRecord(t *testing.T) {
	source := strings.Join([]string{
		"「" + replay.RulesVersion + "」でルール版宣言",
		"「draw-1」と1で対局開始",
		"黒パス",
		"白パス",
		"「引分」で対局終了   # 2手、色合計255、駒2",
	}, "\n") + "\n"

	decoder, _ := newFakeDecoder(t, drawScriptOutput(t))
	replayer, err := replay.NewReplayer(decoder)
	if err != nil {
		t.Fatalf("replayerを作れません: %v", err)
	}

	result, err := replayer.Replay(context.Background(), source)
	if err != nil {
		t.Fatalf("引き分けの棋譜を再生できません: %v", err)
	}
	if result.Script.Winner != nil {
		t.Errorf("引き分けなのに勝者がいます: %v", result.Script.Winner)
	}
	final := result.FinalState()
	if final.Phase != protocol.PhaseFinished || final.Winner != nil {
		t.Errorf("終局が引き分けになっていません: phase=%s winner=%v", final.Phase, final.Winner)
	}
	if record := result.Record(); record.Winner != nil {
		t.Errorf("対局記録の勝者が引き分けになっていません: %v", record.Winner)
	}

	// 書き出しても引分の終了行になり、そのまま読み直せること
	encoded, err := replay.EncodeResult(result)
	if err != nil {
		t.Fatalf("引き分けの棋譜を書き出せません: %v", err)
	}
	if !strings.Contains(encoded, "「引分」で対局終了") {
		t.Errorf("引分の終了行がありません: %q", encoded)
	}
	outline, err := replay.Scan(encoded)
	if err != nil {
		t.Fatalf("書き出した棋譜を走査できません: %v", err)
	}
	if outline.FooterLines != 1 || len(outline.MoveLines) != 2 {
		t.Errorf("往復した棋譜の構成が違います: %+v", outline)
	}

	again, err := replayer.Replay(context.Background(), encoded)
	if err != nil {
		t.Fatalf("書き出した棋譜を再生できません: %v", err)
	}
	if again.FinalState().Winner != nil {
		t.Error("往復後に勝者が生まれています")
	}
}
