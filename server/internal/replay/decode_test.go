package replay_test

import (
	"context"
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
		{name: "開始行がない", source: "「1」でルール版宣言\n2と2で黒着手\n"},
		{
			name:   "開始行が2回ある",
			source: "「1」でルール版宣言\n「demo-1」と1で対局開始\n「demo-2」と2で対局開始\n2と2で黒着手\n",
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
	source := "「1」でルール版宣言\n「demo-1」と1で対局開始\n2と2で黒着手\n"
	decoder, runner := newFakeDecoder(t,
		`{"version":"1","rulesVersion":"1","ok":true,"gameId":"demo-1","seed":1,"winner":"","moves":[{"player":"dark","type":"place","row":2,"col":2}],"headerCount":1,"footerCount":0}`)

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
	source := "「1」でルール版宣言\n「demo 1」と1で対局開始\n2と2で黒着手\n"
	decoder, _ := newFakeDecoder(t,
		`{"version":"1","rulesVersion":"1","ok":true,"gameId":"demo 1","seed":1,"winner":"","moves":[{"player":"dark","type":"place","row":2,"col":2}],"headerCount":1,"footerCount":0}`)

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
	source := "「1」でルール版宣言\n「demo-1」と1で対局開始\n2と2で黒着手\n白パス\n"
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

	end, err := replay.EndLine(protocol.PlayerLight, 61, 8891, 64)
	if err != nil {
		t.Fatalf("終了行を作れません: %v", err)
	}
	if !strings.HasPrefix(end, "「白」で対局終了") || !strings.Contains(end, "# 61手、色合計8891、駒64") {
		t.Errorf("終了行が違います: %q", end)
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
