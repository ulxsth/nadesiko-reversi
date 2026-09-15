package replay_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ulxsth/nadesiko-reversi/server/internal/protocol"
	"github.com/ulxsth/nadesiko-reversi/server/internal/replay"
)

// newEntry はstore検証用の最小entryを作る。
func newEntry(gameID string, seed uint32, winner protocol.Player, moves int) replay.Entry {
	state := finishedState(gameID, winner)
	commands := make([]protocol.Command, 0, moves)
	for i := 0; i < moves; i++ {
		commands = append(commands, replay.PlaceMove(protocol.PlayerDark, 2, 2).Command(i))
	}
	record := replay.Record{
		Version:    replay.Version,
		GameID:     gameID,
		Seed:       seed,
		StartedAt:  "2026-09-12T14:00:00Z",
		EndedAt:    "2026-09-12T14:05:00Z",
		Winner:     winner,
		Commands:   commands,
		FinalState: state,
	}
	source, err := replay.StartLine(gameID, seed)
	if err != nil {
		panic(err)
	}
	return replay.Entry{Record: record, Source: source + "\n"}
}

// finishedState は終局したstateを作る。
func finishedState(gameID string, winner protocol.Player) protocol.State {
	board := protocol.NewBoard()
	for i := range board {
		board[i] = protocol.NewCell(0)
	}
	return protocol.State{
		Version:       protocol.Version,
		GameID:        gameID,
		Board:         board,
		TurnNumber:    60,
		CurrentPlayer: protocol.PlayerDark,
		NextColor:     7,
		RNGState:      1234,
		Phase:         protocol.PhaseFinished,
		Winner:        &winner,
		LegalMoves:    []protocol.Position{},
	}
}

func TestMemoryStoreListKeepsInsertionOrder(t *testing.T) {
	ctx := context.Background()
	store := replay.NewMemoryStore()

	for _, gameID := range []string{"game-c", "game-a", "game-b"} {
		if err := store.Save(ctx, newEntry(gameID, 1, protocol.PlayerDark, 2)); err != nil {
			t.Fatalf("保存に失敗: %v", err)
		}
	}

	summaries, err := store.List(ctx)
	if err != nil {
		t.Fatalf("一覧に失敗: %v", err)
	}
	want := []string{"game-c", "game-a", "game-b"}
	if len(summaries) != len(want) {
		t.Fatalf("件数が違います: got %d, want %d", len(summaries), len(want))
	}
	for i, gameID := range want {
		if summaries[i].GameID != gameID {
			t.Errorf("一覧[%d]が違います: got %q, want %q", i, summaries[i].GameID, gameID)
		}
	}
	if summaries[0].MoveCount != 2 {
		t.Errorf("手数が違います: %d", summaries[0].MoveCount)
	}
}

func TestMemoryStoreSaveOverwritesWithoutDuplicating(t *testing.T) {
	ctx := context.Background()
	store := replay.NewMemoryStore()

	if err := store.Save(ctx, newEntry("game-a", 1, protocol.PlayerDark, 2)); err != nil {
		t.Fatalf("保存に失敗: %v", err)
	}
	if err := store.Save(ctx, newEntry("game-a", 9, protocol.PlayerLight, 5)); err != nil {
		t.Fatalf("上書きに失敗: %v", err)
	}

	if store.Len() != 1 {
		t.Errorf("件数が増えました: %d", store.Len())
	}
	entry, err := store.Get(ctx, "game-a")
	if err != nil {
		t.Fatalf("取得に失敗: %v", err)
	}
	if entry.Record.Seed != 9 || entry.Record.Winner != protocol.PlayerLight {
		t.Errorf("上書きされていません: %+v", entry.Record.Summary())
	}
}

func TestMemoryStoreGetReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	store := replay.NewMemoryStore()

	_, err := store.Get(ctx, "missing")
	var notFound *replay.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("NotFoundErrorではありません: %T %v", err, err)
	}
	if notFound.GameID != "missing" {
		t.Errorf("gameIdが記録されていません: %q", notFound.GameID)
	}
}

func TestMemoryStoreRandomPicksFromSaved(t *testing.T) {
	ctx := context.Background()

	empty := replay.NewMemoryStore()
	if _, err := empty.Random(ctx); err == nil {
		t.Fatal("空の保管庫でエラーになりません")
	}

	// 乱数を固定して、選択が保存順のindexに対応することを確認する
	picked := 0
	store := replay.NewMemoryStore().WithRandom(func(int) int { return picked })
	for _, gameID := range []string{"game-a", "game-b", "game-c"} {
		if err := store.Save(ctx, newEntry(gameID, 1, protocol.PlayerDark, 1)); err != nil {
			t.Fatalf("保存に失敗: %v", err)
		}
	}

	for index, want := range []string{"game-a", "game-b", "game-c"} {
		picked = index
		entry, err := store.Random(ctx)
		if err != nil {
			t.Fatalf("ランダム取得に失敗: %v", err)
		}
		if entry.Record.GameID != want {
			t.Errorf("index %d の取得が違います: got %q, want %q", index, entry.Record.GameID, want)
		}
	}
}

func TestMemoryStoreReturnsIndependentCopies(t *testing.T) {
	ctx := context.Background()
	store := replay.NewMemoryStore()
	if err := store.Save(ctx, newEntry("game-a", 1, protocol.PlayerDark, 2)); err != nil {
		t.Fatalf("保存に失敗: %v", err)
	}

	first, err := store.Get(ctx, "game-a")
	if err != nil {
		t.Fatalf("取得に失敗: %v", err)
	}
	first.Record.GameID = "改ざん"
	first.Record.Commands[0].CommandID = "改ざん"
	*first.Record.FinalState.Board[0] = 200

	second, err := store.Get(ctx, "game-a")
	if err != nil {
		t.Fatalf("取得に失敗: %v", err)
	}
	if second.Record.GameID != "game-a" {
		t.Error("保管庫のgameIdが書き換わりました")
	}
	if second.Record.Commands[0].CommandID == "改ざん" {
		t.Error("保管庫のcommandが書き換わりました")
	}
	if got := *second.Record.FinalState.Board[0]; got != 0 {
		t.Errorf("保管庫の盤面が書き換わりました: %d", got)
	}
}
