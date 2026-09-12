## ゴール

確定した対局を実行可能ななでしこコードとして保存し、再実行によって各手の盤面を復元できるリプレイ機能を作る。元実装のWhitespace変換は移植せず、「日本語コードそのものが棋譜」という見せ場へ置き換える。

## Agent assignment contract

- change-id: `add-executable-replay`
- branch: `feature/add-executable-replay`
- 専有パス:
  - `server/internal/replay/**`
  - `rules/replay/**`
  - `openspec/changes/add-executable-replay/**`
- 変更禁止:
  - `server/main.go`
  - `web/**`
  - `rules/game/**`
  - `openspec/specs/**`
  - 共有設定ファイル
- 依存Issue: #1、#2、#3（すべて`develop`へマージ済みであること）

## 受け入れ条件

- [ ] 対局メタデータと全着手をなでしこソースへ直列化できる
- [ ] 生成ソースをgonakoで実行し、各手または指定手数の盤面を復元できる
- [ ] 同じ棋譜から最終盤面と勝者が一致する
- [ ] 不正・改ざん棋譜を行番号つきエラーで拒否する
- [ ] P0ではプロセス内保存でよく、保存interfaceは永続化実装へ差し替え可能にする
- [ ] 一覧、ID取得、ランダム1件取得のservice testがある
- [ ] サンプル棋譜が人間に読める日本語コードになっている
- [ ] OpenSpecの全artifactが揃い、WSLで`make check`が成功する

## マージ条件

UIとrouteへの配線は#7へ残す。feature PRではarchiveしない。
