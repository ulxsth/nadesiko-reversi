package replay

import (
	"context"
	"fmt"
)

// Service は棋譜の保存・取得・再生をまとめる。
// #7はこのserviceだけを呼べばよく、gonakoの起動もstoreの実装も見えない。
type Service struct {
	store    Store
	replayer *Replayer
}

// NewService はserviceを組み立てる。
func NewService(store Store, replayer *Replayer) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("storeが必要です")
	}
	if replayer == nil {
		return nil, fmt.Errorf("replayerが必要です")
	}
	return &Service{store: store, replayer: replayer}, nil
}

// SaveSource は棋譜を再生して検証し、通ったものだけを保存する。
// 改ざんされた棋譜はここで行番号つきに落ちるため、保管庫へ入らない。
func (s *Service) SaveSource(ctx context.Context, source string) (*Record, error) {
	result, err := s.replayer.Replay(ctx, source)
	if err != nil {
		return nil, err
	}

	entry := Entry{Record: result.Record(), Source: source}
	if err := s.store.Save(ctx, entry); err != nil {
		return nil, err
	}
	record := entry.Record
	return &record, nil
}

// SaveResult は確定した対局を棋譜へ直列化して保存する。
func (s *Service) SaveResult(ctx context.Context, result *Result) (string, error) {
	source, err := EncodeResult(result)
	if err != nil {
		return "", err
	}
	if err := s.store.Save(ctx, Entry{Record: result.Record(), Source: source}); err != nil {
		return "", err
	}
	return source, nil
}

// List は保存済みの対局の要約を返す。
func (s *Service) List(ctx context.Context) ([]Summary, error) {
	return s.store.List(ctx)
}

// Get はgameIdで対局を1件返す。
func (s *Service) Get(ctx context.Context, gameID string) (*Record, error) {
	entry, err := s.store.Get(ctx, gameID)
	if err != nil {
		return nil, err
	}
	record := entry.Record
	return &record, nil
}

// Random は保存済みの対局から1件返す。デモの導線で使う。
func (s *Service) Random(ctx context.Context) (*Record, error) {
	entry, err := s.store.Random(ctx)
	if err != nil {
		return nil, err
	}
	record := entry.Record
	return &record, nil
}

// Source はgameIdの棋譜を、実行可能ななでしこコードとして返す。
func (s *Service) Source(ctx context.Context, gameID string) (string, error) {
	entry, err := s.store.Get(ctx, gameID)
	if err != nil {
		return "", err
	}
	return entry.Source, nil
}

// Replay はgameIdの棋譜を再生し、各手の盤面を返す。
func (s *Service) Replay(ctx context.Context, gameID string) (*Result, error) {
	source, err := s.Source(ctx, gameID)
	if err != nil {
		return nil, err
	}
	return s.replayer.Replay(ctx, source)
}

// ReplayUntil はgameIdの棋譜を指定手数まで再生する。0は初期局面。
func (s *Service) ReplayUntil(ctx context.Context, gameID string, moveNumber int) (*Result, error) {
	source, err := s.Source(ctx, gameID)
	if err != nil {
		return nil, err
	}
	return s.replayer.ReplayUntil(ctx, source, moveNumber)
}
