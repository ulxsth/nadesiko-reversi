package replay_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/replay"
	"github.com/ulxsth/nadesiko-reversi/server/internal/runtime"
)

// sampleRecordPath は人が読むためのサンプル棋譜。
// UPDATE_SAMPLE=1 を付けて実行すると実対局から作り直す。
const sampleRecordName = "sample-game.nako3"

func repoPath(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
}

func gonakoPath() string {
	if custom := os.Getenv("GONAKO_BIN"); custom != "" {
		return custom
	}
	return repoPath(".tools", "bin", "gonako")
}

func requireGonako(t *testing.T) string {
	t.Helper()
	binary := gonakoPath()
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("gonakoが見つからないためskipします (%s): make bootstrapが必要です", binary)
	}
	return binary
}

// newRuleRunner は#3のruntime adapterを実ルールengineへ向けて作る。
func newRuleRunner(t *testing.T) runtime.Runner {
	t.Helper()
	runner, err := runtime.New(runtime.Config{
		GonakoPath: requireGonako(t),
		RulesPath:  repoPath("rules", "game", "main.nako3"),
		Timeout:    30 * time.Second,
	})
	if err != nil {
		t.Fatalf("rule runnerを作れません: %v", err)
	}
	return runner
}

// newReplayer は再生ハーネスを読み込んでreplayerを作る。
func newReplayer(t *testing.T) *replay.Replayer {
	t.Helper()

	scriptRunner, err := replay.NewGonakoScriptRunner(requireGonako(t), 30*time.Second)
	if err != nil {
		t.Fatalf("script runnerを作れません: %v", err)
	}
	harness, err := replay.LoadHarness(repoPath("rules", "replay", "harness.nako3"))
	if err != nil {
		t.Fatalf("harnessを読めません: %v", err)
	}
	decoder, err := replay.NewDecoder(harness, scriptRunner)
	if err != nil {
		t.Fatalf("decoderを作れません: %v", err)
	}
	replayer, err := replay.NewReplayer(decoder)
	if err != nil {
		t.Fatalf("replayerを作れません: %v", err)
	}
	return replayer
}

// playGame は実ルールengineで対局を最後まで進め、棋譜ソースを組み立てる。
// 手の選択は固定seedのLCGで決めるので、同じ入力からは同じ棋譜になる。
func playGame(t *testing.T, ruleRunner runtime.Runner, gameID string, seed uint32) string {
	t.Helper()
	ctx := context.Background()

	response, err := runtime.NewGame(ctx, ruleRunner, gameID, seed)
	if err != nil {
		t.Fatalf("newGameに失敗: %v", err)
	}
	if !response.OK || response.State == nil {
		t.Fatalf("newGameが拒否されました: %+v", response.Error)
	}
	state := *response.State

	var lines []string
	lines = append(lines, replay.HeaderLines("2026-09-12T21:30:00+09:00", "2026-09-12T21:44:12+09:00")...)
	lines = append(lines, "")

	startLine, err := replay.StartLine(gameID, seed)
	if err != nil {
		t.Fatalf("開始行を作れません: %v", err)
	}
	lines = append(lines, replay.RulesVersionLine())
	lines = append(lines, startLine, "")

	pick := uint64(20260912)
	nextPick := func(limit int) int {
		pick = (pick*6364136223846793005 + 1442695040888963407) % (1 << 62)
		return int(pick % uint64(limit))
	}

	for turn := 0; state.Phase == protocol.PhasePlaying && turn < 200; turn++ {
		var command protocol.Command
		if len(state.LegalMoves) == 0 {
			command = protocol.PassCommand(fmt.Sprintf("play-%d", turn+1), state.CurrentPlayer, state.TurnNumber)
		} else {
			move := state.LegalMoves[nextPick(len(state.LegalMoves))]
			command = protocol.PlaceCommand(fmt.Sprintf("play-%d", turn+1), state.CurrentPlayer, state.TurnNumber, move.Row, move.Col)
		}

		applied, err := runtime.ApplyCommand(ctx, ruleRunner, state, command)
		if err != nil {
			t.Fatalf("%d手目の適用に失敗: %v", turn+1, err)
		}
		if !applied.OK || applied.State == nil {
			t.Fatalf("%d手目が拒否されました: %+v", turn+1, applied.Error)
		}

		var line string
		if command.Type == protocol.CommandPass {
			line, err = replay.PassLine(command.Player)
		} else {
			placedColor := uint8(0)
			changed := 0
			if applied.Event != nil {
				if applied.Event.PlacedColor != nil {
					placedColor = *applied.Event.PlacedColor
				}
				changed = len(applied.Event.Changes)
			}
			line, err = replay.PlaceLine(command.Player, *command.Row, *command.Col, placedColor, changed)
		}
		if err != nil {
			t.Fatalf("%d手目を書き出せません: %v", turn+1, err)
		}
		lines = append(lines, line)
		state = *applied.State
	}

	// 引き分けならwinnerはnilのままなので、終局したかだけを見る
	if state.Phase != protocol.PhaseFinished {
		t.Fatalf("対局が終わりませんでした: phase=%s", state.Phase)
	}

	colorSum, pieceCount := 0, 0
	for _, cell := range state.Board {
		if cell == nil {
			continue
		}
		colorSum += int(*cell)
		pieceCount++
	}
	endLine, err := replay.EndLine(state.Winner, state.TurnNumber, colorSum, pieceCount)
	if err != nil {
		t.Fatalf("終了行を作れません: %v", err)
	}
	lines = append(lines, "", endLine)

	return strings.Join(lines, "\n") + "\n"
}

// sampleSource はサンプル棋譜を読む。UPDATE_SAMPLE=1なら実対局から作り直す。
func sampleSource(t *testing.T) string {
	t.Helper()
	path := repoPath("rules", "replay", sampleRecordName)

	if os.Getenv("UPDATE_SAMPLE") == "1" {
		source := playGame(t, newRuleRunner(t), "sample-game", 1)
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("サンプル棋譜を書けません: %v", err)
		}
		t.Logf("サンプル棋譜を更新しました: %s", path)
		return source
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("サンプル棋譜を読めません (UPDATE_SAMPLE=1で生成できます): %v", err)
	}
	return string(data)
}

func TestHarnessReadsContractExample(t *testing.T) {
	replayer := newReplayer(t)

	// docs/contracts/game-record.md の例をそのまま読む
	source := strings.Join([]string{
		"# グラデーションリバーシ 対局記録 v1",
		"# 開始 2026-09-12T21:30:00+09:00",
		"# 終了 2026-09-12T21:44:12+09:00",
		"",
		"「2」でルール版宣言",
		"「demo-1」と1で対局開始",
		"",
		"2と5で黒着手    # 色60、1個変換",
		"5と4で白着手    # 色94、1個変換",
		"6と4で黒着手    # 色129、2個変換",
		"# …（中略）…",
		"",
		"「白」で対局終了   # 61手、色合計8891、駒64",
	}, "\n")

	// 途中局面までの再生なので終局と注釈は確かめない。
	// 合法性は棋譜全体ぶん確かめられる（再生は常に1プロセスで最後まで走る）。
	result, err := replayer.ReplayUntil(context.Background(), source, 0)
	if err != nil {
		t.Fatalf("契約の例を読めません: %v", err)
	}

	if result.Script.GameID != "demo-1" {
		t.Errorf("gameIdが違います: %q", result.Script.GameID)
	}
	if result.Script.Seed != 1 {
		t.Errorf("seedが違います: %d", result.Script.Seed)
	}
	if result.Script.Winner == nil || *result.Script.Winner != protocol.PlayerLight {
		t.Errorf("勝者が違います: %v", result.Script.Winner)
	}
	if got := len(result.Script.Moves); got != 3 {
		t.Fatalf("手数が違います: %d", got)
	}
	if result.Script.Moves[0].Player != protocol.PlayerDark || *result.Script.Moves[0].Row != 2 {
		t.Errorf("1手目が違います: %+v", result.Script.Moves[0])
	}
	if result.Script.Moves[2].Player != protocol.PlayerDark || *result.Script.Moves[2].Col != 4 {
		t.Errorf("3手目が違います: %+v", result.Script.Moves[2])
	}
	if result.Script.StartedAt != "2026-09-12T21:30:00+09:00" {
		t.Errorf("開始時刻をヘッダから読めていません: %q", result.Script.StartedAt)
	}

	// 注釈が行番号つきで対応づいていること
	if got := len(result.Outline.MoveLines); got != 3 {
		t.Fatalf("行の対応が違います: %d", got)
	}
	first := result.Outline.MoveLines[0]
	if !first.Annotated || first.Color != 60 || first.Changed != 1 {
		t.Errorf("注釈を読めていません: %+v", first)
	}
	if first.Line != 8 {
		t.Errorf("行番号が違います: %d", first.Line)
	}
}

func TestSampleRecordReplaysToRecordedResult(t *testing.T) {
	replayer := newReplayer(t)
	source := sampleSource(t)

	result, err := replayer.Replay(context.Background(), source)
	if err != nil {
		t.Fatalf("サンプル棋譜を再生できません: %v", err)
	}

	final := result.FinalState()
	if final.Phase != protocol.PhaseFinished {
		t.Errorf("再生後に終局していません: %s", final.Phase)
	}
	if !samePlayerForTest(final.Winner, result.Script.Winner) {
		t.Errorf("勝者が一致しません: %v", final.Winner)
	}
	if len(result.Frames) != len(result.Script.Moves)+1 {
		t.Errorf("盤面の数が違います: %d (手数%d)", len(result.Frames), len(result.Script.Moves))
	}

	// 各手の盤面を取り出せること
	mid := len(result.Script.Moves) / 2
	frame, ok := result.FrameAt(mid)
	if !ok {
		t.Fatalf("%d手目の盤面を取れません", mid)
	}
	if frame.TurnNumber != mid {
		t.Errorf("%d手目のturnNumberが違います: %d", mid, frame.TurnNumber)
	}
	if frame.Line == 0 {
		t.Errorf("%d手目に行番号が付いていません", mid)
	}

	initial, _ := result.FrameAt(0)
	pieces := 0
	for _, cell := range initial.State.Board {
		if cell != nil {
			pieces++
		}
	}
	if pieces != 4 {
		t.Errorf("初期局面の駒数が違います: %d", pieces)
	}
}

func TestSampleRecordRoundTripsThroughEncoder(t *testing.T) {
	replayer := newReplayer(t)
	source := sampleSource(t)

	result, err := replayer.Replay(context.Background(), source)
	if err != nil {
		t.Fatalf("再生に失敗: %v", err)
	}

	encoded, err := replay.EncodeResult(result)
	if err != nil {
		t.Fatalf("棋譜を書き出せません: %v", err)
	}
	if strings.TrimSpace(encoded) != strings.TrimSpace(source) {
		t.Errorf("再生結果から作り直した棋譜が元と違います\n--- 元 ---\n%s\n--- 往復 ---\n%s",
			strings.TrimSpace(source), strings.TrimSpace(encoded))
	}
}

func TestTamperedRecordIsRejectedWithLine(t *testing.T) {
	replayer := newReplayer(t)
	source := sampleSource(t)
	lines := strings.Split(strings.TrimRight(source, "\n"), "\n")

	// 最初の着手行を、挟みが成立しない隅へ書き換える
	target := -1
	for i, line := range lines {
		if strings.Contains(line, "着手") {
			target = i
			break
		}
	}
	if target < 0 {
		t.Fatal("着手行が見つかりません")
	}
	original := lines[target]
	lines[target] = "0と0で黒着手    # 色60、3個変換"

	_, err := replayer.Replay(context.Background(), strings.Join(lines, "\n")+"\n")
	if err == nil {
		t.Fatal("改ざんされた棋譜が受理されました")
	}

	var sourceErr *replay.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("SourceErrorではありません: %T %v", err, err)
	}
	if sourceErr.Code != replay.CodeRecordIllegalMove {
		t.Errorf("codeが違います: %q", sourceErr.Code)
	}
	if sourceErr.Line != target+1 {
		t.Errorf("行番号が違います: got %d, want %d (元の行: %s)", sourceErr.Line, target+1, original)
	}
	if sourceErr.MoveNumber != 1 {
		t.Errorf("手数が違います: %d", sourceErr.MoveNumber)
	}
	if sourceErr.RuleCode != protocol.CodeIllegalMove {
		t.Errorf("ルール側のcodeが違います: %q", sourceErr.RuleCode)
	}
	if !strings.Contains(sourceErr.Error(), "0と0で黒着手") {
		t.Errorf("エラーに該当行のソースが含まれていません: %s", sourceErr.Error())
	}
}

func TestTamperedAnnotationIsRejected(t *testing.T) {
	replayer := newReplayer(t)
	source := sampleSource(t)
	lines := strings.Split(strings.TrimRight(source, "\n"), "\n")

	target := -1
	for i, line := range lines {
		if strings.Contains(line, "着手") && strings.Contains(line, "色") {
			target = i
			break
		}
	}
	if target < 0 {
		t.Fatal("注釈つきの着手行が見つかりません")
	}
	// 手そのものは変えず、注釈の色だけを書き換える
	lines[target] = strings.Replace(lines[target], "# 色", "# 色9", 1)

	_, err := replayer.Replay(context.Background(), strings.Join(lines, "\n")+"\n")
	if err == nil {
		t.Fatal("注釈が食い違う棋譜が受理されました")
	}

	var sourceErr *replay.SourceError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("SourceErrorではありません: %T %v", err, err)
	}
	if sourceErr.Code != replay.CodeRecordMismatch {
		t.Errorf("codeが違います: %q (%s)", sourceErr.Code, sourceErr.Message)
	}
	if sourceErr.Line != target+1 {
		t.Errorf("行番号が違います: got %d, want %d", sourceErr.Line, target+1)
	}
}

func TestServiceListGetRandom(t *testing.T) {
	ctx := context.Background()
	replayer := newReplayer(t)

	picked := 0
	store := replay.NewMemoryStore().WithRandom(func(int) int { return picked })
	service, err := replay.NewService(store, replayer)
	if err != nil {
		t.Fatalf("serviceを作れません: %v", err)
	}

	source := sampleSource(t)
	record, err := service.SaveSource(ctx, source)
	if err != nil {
		t.Fatalf("棋譜を保存できません: %v", err)
	}
	if record.GameID != "sample-game" {
		t.Errorf("gameIdが違います: %q", record.GameID)
	}

	summaries, err := service.List(ctx)
	if err != nil {
		t.Fatalf("一覧に失敗: %v", err)
	}
	if len(summaries) != 1 || summaries[0].GameID != "sample-game" {
		t.Fatalf("一覧が違います: %+v", summaries)
	}
	if summaries[0].MoveCount != len(record.Commands) {
		t.Errorf("手数が違います: %d", summaries[0].MoveCount)
	}

	got, err := service.Get(ctx, "sample-game")
	if err != nil {
		t.Fatalf("取得に失敗: %v", err)
	}
	if !samePlayerForTest(got.Winner, record.Winner) {
		t.Errorf("勝者が違います: %v", got.Winner)
	}

	random, err := service.Random(ctx)
	if err != nil {
		t.Fatalf("ランダム取得に失敗: %v", err)
	}
	if random.GameID != "sample-game" {
		t.Errorf("ランダム取得が違います: %q", random.GameID)
	}

	stored, err := service.Source(ctx, "sample-game")
	if err != nil {
		t.Fatalf("棋譜ソースを取れません: %v", err)
	}
	if stored != source {
		t.Error("保存した棋譜ソースが元と違います")
	}

	// 指定手数の盤面を取り出せること
	partial, err := service.ReplayUntil(ctx, "sample-game", 3)
	if err != nil {
		t.Fatalf("途中まで再生できません: %v", err)
	}
	if len(partial.Frames) != 4 {
		t.Errorf("盤面の数が違います: %d", len(partial.Frames))
	}
}

// samePlayerForTest は勝者が一致するかを返す。どちらもnilなら引き分けどうしで一致。
func samePlayerForTest(left, right *protocol.Player) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// TestRulesReturnDrawForBalancedBoard は色の重心がちょうど中央の終局が
// 引き分けになることを、実ルールengineを動かして確かめる。
//
// (0,0)=0と(7,7)=255だけの盤面はどの空きマスからも駒を挟めないので合法手が
// 0件になり、色合計255・駒2で`2*255 == 2*255`が成り立つ。
func TestRulesReturnDrawForBalancedBoard(t *testing.T) {
	ctx := context.Background()
	ruleRunner := newRuleRunner(t)

	board := protocol.NewBoard()
	board[0] = protocol.NewCell(0)
	board[63] = protocol.NewCell(255)
	state := protocol.State{
		Version:       protocol.Version,
		GameID:        "draw-board",
		Board:         board,
		TurnNumber:    0,
		CurrentPlayer: protocol.PlayerDark,
		NextColor:     7,
		RNGState:      1234,
		Phase:         protocol.PhasePlaying,
		LegalMoves:    []protocol.Position{},
	}

	first, err := runtime.ApplyCommand(ctx, ruleRunner, state,
		protocol.PassCommand("draw-pass-1", protocol.PlayerDark, 0))
	if err != nil {
		t.Fatalf("1回目のパスに失敗: %v", err)
	}
	if !first.OK || first.State == nil {
		t.Fatalf("1回目のパスが拒否されました: %+v", first.Error)
	}

	second, err := runtime.ApplyCommand(ctx, ruleRunner, *first.State,
		protocol.PassCommand("draw-pass-2", protocol.PlayerLight, 1))
	if err != nil {
		t.Fatalf("2回目のパスに失敗: %v", err)
	}
	if !second.OK || second.State == nil {
		t.Fatalf("2回目のパスが拒否されました: %+v", second.Error)
	}

	if second.State.Phase != protocol.PhaseFinished {
		t.Fatalf("連続パスで終局しません: phase=%s", second.State.Phase)
	}
	if second.State.Winner != nil {
		t.Errorf("引き分けなのに勝者がいます: %q", string(*second.State.Winner))
	}
	if err := second.State.Validate(); err != nil {
		t.Errorf("引き分けのstateが不正と判定されました: %v", err)
	}
}

// TestRulesRoundColorTransform は色変換の丸めを実ルールengineで固定する。
//
// (0,1)を変換対象、(0,2)を固定端にして(0,0)へ置くと、変換色は
// round((nextColor+固定端+変換対象)/3)になる。3で割った余り0/1/2と、
// 和が0と765の両端を確かめる。
func TestRulesRoundColorTransform(t *testing.T) {
	ctx := context.Background()
	ruleRunner := newRuleRunner(t)

	tests := []struct {
		name      string
		nextColor uint8
		target    uint8
		anchor    uint8
		want      uint8
	}{
		{name: "余り0", nextColor: 0, target: 0, anchor: 0, want: 0},
		{name: "余り1", nextColor: 1, target: 0, anchor: 0, want: 0},
		{name: "余り2", nextColor: 2, target: 0, anchor: 0, want: 1},
		{name: "割り切れる", nextColor: 3, target: 0, anchor: 0, want: 1},
		{name: "上端", nextColor: 255, target: 255, anchor: 255, want: 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := protocol.NewBoard()
			board[1] = protocol.NewCell(tt.target)
			board[2] = protocol.NewCell(tt.anchor)
			state := protocol.State{
				Version:       protocol.Version,
				GameID:        "round-board",
				Board:         board,
				TurnNumber:    0,
				CurrentPlayer: protocol.PlayerDark,
				NextColor:     tt.nextColor,
				RNGState:      1234,
				Phase:         protocol.PhasePlaying,
				LegalMoves:    []protocol.Position{{Row: 0, Col: 0}},
			}

			applied, err := runtime.ApplyCommand(ctx, ruleRunner, state,
				protocol.PlaceCommand("round-1", protocol.PlayerDark, 0, 0, 0))
			if err != nil {
				t.Fatalf("着手に失敗: %v", err)
			}
			if !applied.OK || applied.State == nil {
				t.Fatalf("着手が拒否されました: %+v", applied.Error)
			}
			got := applied.State.Board[1]
			if got == nil {
				t.Fatal("変換対象が空きマスになっています")
			}
			if *got != tt.want {
				t.Errorf("変換色が違います: got %d, want %d", *got, tt.want)
			}
		})
	}
}

// TestRulesReturnDrawWhenBoardFills は盤面満杯の終了経路でも引き分けになることを確かめる。
//
// 着手(0,0)で変換されるのは行0・列0・対角の駒だけなので、そこと置く駒を0にして
// 変換に関与しない42マスのうち32マスを255にすると、着手後の色合計は8160・駒64になり
// `2*8160 == 64*255`が成り立つ。
func TestRulesReturnDrawWhenBoardFills(t *testing.T) {
	ctx := context.Background()
	ruleRunner := newRuleRunner(t)

	board := protocol.NewBoard()
	whites := 0
	for index := range board {
		if index == 0 {
			// 最後の空きマス
			continue
		}
		transformed := index <= 7 || index%8 == 0 || index/8 == index%8
		color := uint8(0)
		if !transformed && whites < 32 {
			color = 255
			whites++
		}
		board[index] = protocol.NewCell(color)
	}
	if whites != 32 {
		t.Fatalf("白駒の数が違います: %d", whites)
	}

	state := protocol.State{
		Version:       protocol.Version,
		GameID:        "draw-full",
		Board:         board,
		TurnNumber:    0,
		CurrentPlayer: protocol.PlayerDark,
		NextColor:     0,
		RNGState:      1234,
		Phase:         protocol.PhasePlaying,
		LegalMoves:    []protocol.Position{{Row: 0, Col: 0}},
	}

	applied, err := runtime.ApplyCommand(ctx, ruleRunner, state,
		protocol.PlaceCommand("draw-full-1", protocol.PlayerDark, 0, 0, 0))
	if err != nil {
		t.Fatalf("最後の着手に失敗: %v", err)
	}
	if !applied.OK || applied.State == nil {
		t.Fatalf("最後の着手が拒否されました: %+v", applied.Error)
	}
	if applied.State.Phase != protocol.PhaseFinished {
		t.Fatalf("盤面満杯で終局しません: phase=%s", applied.State.Phase)
	}
	if applied.State.Winner != nil {
		t.Errorf("引き分けなのに勝者がいます: %q", string(*applied.State.Winner))
	}

	colorSum := 0
	for _, cell := range applied.State.Board {
		if cell == nil {
			t.Fatal("盤面が満杯になっていません")
		}
		colorSum += int(*cell)
	}
	if colorSum != 8160 {
		t.Errorf("色合計が違います: got %d, want 8160", colorSum)
	}
}
