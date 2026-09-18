package replay_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ulxsth/nadesiko-reversi/server/internal/replay"
)

// countingScriptRunner は実runnerへ委譲しつつ起動回数を数える。
type countingScriptRunner struct {
	inner replay.ScriptRunner
	runs  int
}

func (c *countingScriptRunner) Run(ctx context.Context, source string) ([]byte, error) {
	c.runs++
	return c.inner.Run(ctx, source)
}

// newCountingReplayer は起動回数を数えられるreplayerを作る。
func newCountingReplayer(t *testing.T) (*replay.Replayer, *countingScriptRunner) {
	t.Helper()

	scriptRunner, err := replay.NewGonakoScriptRunner(requireGonako(t), 30*time.Second)
	if err != nil {
		t.Fatalf("script runnerを作れません: %v", err)
	}
	counter := &countingScriptRunner{inner: scriptRunner}

	harness, err := replay.LoadHarness(repoPath("rules", "replay", "harness.nako3"))
	if err != nil {
		t.Fatalf("harnessを読めません: %v", err)
	}
	decoder, err := replay.NewDecoder(harness, counter)
	if err != nil {
		t.Fatalf("decoderを作れません: %v", err)
	}
	replayer, err := replay.NewReplayer(decoder)
	if err != nil {
		t.Fatalf("replayerを作れません: %v", err)
	}
	return replayer, counter
}

// TestReplaySpawnsExactlyOneProcess は60手の棋譜でもgonakoの起動が1回で済むことを確かめる。
//
// 手ごとにruntimeを呼んでいた頃は手数+2回起動していた。AWSへ移したときに
// リクエストごとのプロセス爆発を起こさないため、ここは厳密に1回で固定する。
func TestReplaySpawnsExactlyOneProcess(t *testing.T) {
	replayer, counter := newCountingReplayer(t)
	source := sampleSource(t)

	result, err := replayer.Replay(context.Background(), source)
	if err != nil {
		t.Fatalf("サンプル棋譜を再生できません: %v", err)
	}

	if moves := len(result.Script.Moves); moves != 60 {
		t.Fatalf("代表棋譜は60手である前提です: %d手", moves)
	}
	if counter.runs != 1 {
		t.Errorf("gonakoの起動回数が違います: got %d, want 1", counter.runs)
	}
	if frames := len(result.Frames); frames != 61 {
		t.Errorf("初期局面を含む盤面数が違います: got %d, want 61", frames)
	}
}

// TestReplayUntilSpawnsExactlyOneProcess は途中手数の取り出しでも起動が増えないことを確かめる。
func TestReplayUntilSpawnsExactlyOneProcess(t *testing.T) {
	replayer, counter := newCountingReplayer(t)
	source := sampleSource(t)

	result, err := replayer.ReplayUntil(context.Background(), source, 10)
	if err != nil {
		t.Fatalf("途中まで再生できません: %v", err)
	}

	if counter.runs != 1 {
		t.Errorf("gonakoの起動回数が違います: got %d, want 1", counter.runs)
	}
	if frames := len(result.Frames); frames != 11 {
		t.Errorf("盤面数が違います: got %d, want 11", frames)
	}
}

// TestReplayRejectsUnsupportedRulesVersion は未対応のルール版を行番号つきで拒否することを確かめる。
// gonakoを起動する前に落とすので、runnerは一度も呼ばれない。
func TestReplayRejectsUnsupportedRulesVersion(t *testing.T) {
	runner := &fakeScriptRunner{output: []byte("{}")}
	decoder, err := replay.NewDecoder("●対局データ出力とは\nここまで\n", runner)
	if err != nil {
		t.Fatalf("decoderを作れません: %v", err)
	}

	source := strings.Join([]string{
		"# グラデーションリバーシ 対局記録 v1",
		"「99」でルール版宣言",
		"「demo-1」と1で対局開始",
		"2と5で黒着手",
	}, "\n")

	_, _, err = decoder.Decode(context.Background(), source)
	var sourceErr *replay.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("SourceErrorではありません: %T %v", err, err)
	}
	if sourceErr.Code != replay.CodeRecordUnsupportedRulesVersion {
		t.Errorf("codeが違います: %q", sourceErr.Code)
	}
	if sourceErr.Line != 2 {
		t.Errorf("行番号が違います: got %d, want 2", sourceErr.Line)
	}
	if sourceErr.Source != "「99」でルール版宣言" {
		t.Errorf("該当行のソースが違います: %q", sourceErr.Source)
	}
	if runner.lastRun != "" {
		t.Error("未対応の版なのにgonakoを起動しています")
	}
}

// TestReplayRejectsMissingRulesVersion は版宣言の無い棋譜を拒否することを確かめる。
func TestReplayRejectsMissingRulesVersion(t *testing.T) {
	runner := &fakeScriptRunner{output: []byte("{}")}
	decoder, err := replay.NewDecoder("●対局データ出力とは\nここまで\n", runner)
	if err != nil {
		t.Fatalf("decoderを作れません: %v", err)
	}

	source := "「demo-1」と1で対局開始\n2と5で黒着手\n"

	_, _, err = decoder.Decode(context.Background(), source)
	var sourceErr *replay.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("SourceErrorではありません: %T %v", err, err)
	}
	if sourceErr.Code != replay.CodeRecordMissingHeader {
		t.Errorf("codeが違います: %q (%s)", sourceErr.Code, sourceErr.Message)
	}
	if runner.lastRun != "" {
		t.Error("版宣言が無いのにgonakoを起動しています")
	}
}

// TestScanRejectsCommandsOutsideVocabulary は許可語彙の外を実行前に弾くことを確かめる。
//
// 棋譜はgonakoで実行するので、許可語彙を素通しにすると任意コード実行になる。
// Scanは許可リストとして働き、4種類の行と注釈以外はすべて文法違反にする。
func TestScanRejectsCommandsOutsideVocabulary(t *testing.T) {
	intrusions := []struct {
		name string
		line string
	}{
		{name: "任意の表示", line: "「のっとり」と表示"},
		{name: "ファイル操作", line: "「/etc/passwd」を開く"},
		{name: "OS実行", line: "「rm -rf /」をOS実行"},
		{name: "変数代入", line: "対局ID=「別対局」"},
		{name: "関数定義", line: "●乗っ取りとは"},
		{name: "取り込み", line: "!「/etc/passwd」を取り込む"},
		{name: "許可語彙の別名", line: "2と5で緑着手"},
	}

	for _, tt := range intrusions {
		t.Run(tt.name, func(t *testing.T) {
			source := "「2」でルール版宣言\n「demo-1」と1で対局開始\n" + tt.line + "\n"

			_, err := replay.Scan(source)
			var sourceErr *replay.SourceError
			if !errors.As(err, &sourceErr) {
				t.Fatalf("許可語彙の外が素通りしています: %T %v", err, err)
			}
			if sourceErr.Code != replay.CodeRecordSyntaxError {
				t.Errorf("codeが違います: %q", sourceErr.Code)
			}
			if sourceErr.Line != 3 {
				t.Errorf("行番号が違います: got %d, want 3", sourceErr.Line)
			}
		})
	}
}

// TestDecodeRejectsIntrusionBeforeRunning は許可語彙の外を含む棋譜でgonakoを起動しないことを確かめる。
func TestDecodeRejectsIntrusionBeforeRunning(t *testing.T) {
	runner := &fakeScriptRunner{output: []byte("{}")}
	decoder, err := replay.NewDecoder("●対局データ出力とは\nここまで\n", runner)
	if err != nil {
		t.Fatalf("decoderを作れません: %v", err)
	}

	source := "「2」でルール版宣言\n「demo-1」と1で対局開始\n「のっとり」と表示\n"

	if _, _, err := decoder.Decode(context.Background(), source); err == nil {
		t.Fatal("許可語彙の外を受け入れています")
	}
	if runner.lastRun != "" {
		t.Error("検査前にgonakoを起動しています")
	}
}

// TestGonakoScriptRunnerCleansTempFiles は一時ファイルを必ず消すことを確かめる。
//
// 読み取り専用のルール配置で動かすため、書き込むのはOSの一時領域だけにする。
// AWSでは/tmpに当たる。
func TestGonakoScriptRunnerCleansTempFiles(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)

	runner, err := replay.NewGonakoScriptRunner(requireGonako(t), 30*time.Second)
	if err != nil {
		t.Fatalf("script runnerを作れません: %v", err)
	}

	if _, err := runner.Run(context.Background(), "「ok」と表示\n"); err != nil {
		t.Fatalf("実行に失敗: %v", err)
	}
	assertTempEmpty(t, tempRoot)

	// 失敗した実行でも消える
	if _, err := runner.Run(context.Background(), "これは文法エラー(((\n"); err == nil {
		t.Fatal("文法エラーが成功扱いです")
	}
	assertTempEmpty(t, tempRoot)
}

// assertTempEmpty は一時領域に棋譜の作業ディレクトリが残っていないことを確かめる。
func assertTempEmpty(t *testing.T, root string) {
	t.Helper()
	leftovers, err := filepath.Glob(filepath.Join(root, "nadesiko-replay-*"))
	if err != nil {
		t.Fatalf("一時領域を調べられません: %v", err)
	}
	if len(leftovers) > 0 {
		t.Errorf("一時ファイルが残っています: %v", leftovers)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("一時領域を読めません: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("一時領域に残骸があります: %d件", len(entries))
	}
}

// TestLoadHarnessResolvesRulesImport は取り込み行が絶対パスへ直ることを確かめる。
func TestLoadHarnessResolvesRulesImport(t *testing.T) {
	harness, err := replay.LoadHarness(repoPath("rules", "replay", "harness.nako3"))
	if err != nil {
		t.Fatalf("harnessを読めません: %v", err)
	}

	if strings.Contains(harness, "!「../game/rules.nako3」を取り込む") {
		t.Error("相対パスのままです。一時領域では解決できません")
	}
	rulesPath, err := filepath.Abs(repoPath("rules", "game", "rules.nako3"))
	if err != nil {
		t.Fatalf("ルール本体のパスを解決できません: %v", err)
	}
	if !strings.Contains(harness, "!「"+rulesPath+"」を取り込む") {
		t.Errorf("絶対パスの取り込み行がありません: %s", rulesPath)
	}
}

// TestRulesLibraryHasNoEntryPoint はルール本体が取り込み時に何も実行しないことを確かめる。
//
// main.nako3は末尾で必ずCLIを実行するため取り込めなかった。分離したrules.nako3は
// 関数定義と定数だけを持ち、単体で走らせても出力しない。
func TestRulesLibraryHasNoEntryPoint(t *testing.T) {
	runner, err := replay.NewGonakoScriptRunner(requireGonako(t), 30*time.Second)
	if err != nil {
		t.Fatalf("script runnerを作れません: %v", err)
	}

	rulesPath, err := filepath.Abs(repoPath("rules", "game", "rules.nako3"))
	if err != nil {
		t.Fatalf("ルール本体のパスを解決できません: %v", err)
	}

	stdout, err := runner.Run(context.Background(), "!「"+rulesPath+"」を取り込む\n")
	if err != nil {
		t.Fatalf("ルール本体を取り込めません: %v", err)
	}
	if got := strings.TrimSpace(string(stdout)); got != "" {
		t.Errorf("取り込みだけで出力しています: %q", got)
	}
}

// BenchmarkReplaySampleRecord は代表的な60手棋譜1本の処理時間を測る。
//
// 1回あたりgonakoを1プロセス起動し、読み取り・全手の適用・各手の盤面生成・
// 終局と注釈の検証までを含む。
func BenchmarkReplaySampleRecord(b *testing.B) {
	binary := gonakoPath()
	if _, err := os.Stat(binary); err != nil {
		b.Skipf("gonakoが見つからないためskipします (%s)", binary)
	}

	scriptRunner, err := replay.NewGonakoScriptRunner(binary, 30*time.Second)
	if err != nil {
		b.Fatalf("script runnerを作れません: %v", err)
	}
	harness, err := replay.LoadHarness(repoPath("rules", "replay", "harness.nako3"))
	if err != nil {
		b.Fatalf("harnessを読めません: %v", err)
	}
	decoder, err := replay.NewDecoder(harness, scriptRunner)
	if err != nil {
		b.Fatalf("decoderを作れません: %v", err)
	}
	replayer, err := replay.NewReplayer(decoder)
	if err != nil {
		b.Fatalf("replayerを作れません: %v", err)
	}

	data, err := os.ReadFile(repoPath("rules", "replay", sampleRecordName))
	if err != nil {
		b.Fatalf("サンプル棋譜を読めません: %v", err)
	}
	source := string(data)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := replayer.Replay(ctx, source); err != nil {
			b.Fatalf("再生に失敗: %v", err)
		}
	}
}

// TestReplayRejectsPreviousRulesVersion は版1の棋譜を拒否することを確かめる。
//
// 版2で色変換の丸めと勝敗判定が変わり、同じ棋譜からでも別の盤面になる。
// 黙って誤再生しないよう、gonakoを起動する前に落とす。
func TestReplayRejectsPreviousRulesVersion(t *testing.T) {
	runner := &fakeScriptRunner{output: []byte("{}")}
	decoder, err := replay.NewDecoder("●対局データ出力とは\nここまで\n", runner)
	if err != nil {
		t.Fatalf("decoderを作れません: %v", err)
	}

	source := strings.Join([]string{
		"「1」でルール版宣言",
		"「demo-1」と1で対局開始",
		"2と5で黒着手",
		"「白」で対局終了",
	}, "\n")

	_, _, err = decoder.Decode(context.Background(), source)
	var sourceErr *replay.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("SourceErrorではありません: %T %v", err, err)
	}
	if sourceErr.Code != replay.CodeRecordUnsupportedRulesVersion {
		t.Errorf("codeが違います: %q", sourceErr.Code)
	}
	if sourceErr.Line != 1 {
		t.Errorf("行番号が違います: got %d, want 1", sourceErr.Line)
	}
	if sourceErr.Source != "「1」でルール版宣言" {
		t.Errorf("該当行のソースが違います: %q", sourceErr.Source)
	}
	if runner.lastRun != "" {
		t.Error("版1なのにgonakoを起動しています")
	}
}
