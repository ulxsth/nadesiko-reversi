## ゴール

ローカルのGoプロセスだけで二人をマッチングし、サーバー権威で手番と盤面を確定して配信する、永続化なしの対戦セッションを実装する。

## Agent assignment contract

- change-id: `add-realtime-match`
- branch: `feature/add-realtime-match`
- 専有パス:
  - `server/internal/match/**`
  - `openspec/changes/add-realtime-match/**`
- 変更禁止:
  - `server/main.go`
  - `server/internal/runtime/**`
  - `server/internal/protocol/**`
  - `web/**`
  - `rules/**`
  - `openspec/specs/**`
  - 共有設定ファイル
- 依存Issue: #1、#3（ともに`develop`へマージ済みであること）

## 受け入れ条件

- [ ] 先着二人を一つのroomへ割り当て、待機とmatch成立イベントを返す
- [ ] ルール評価は#3のinterfaceだけを通して呼ぶ
- [ ] 順番違反、別room、不正手、重複commandを拒否する
- [ ] 確定した盤面、次手番、次の駒色、game phaseを両者へ配信できる
- [ ] 切断時にroomを終了または再接続待ちへ遷移させ、リークしない
- [ ] race detectorで共有状態の競合がない
- [ ] transport非依存のpackage testがある
- [ ] OpenSpecの全artifactが揃い、WSLで`make check`が成功する

## マージ条件

HTTP/WebSocketへの具体的な登録は行わず、#7が呼べるhandler/service境界を公開する。feature PRではarchiveしない。
