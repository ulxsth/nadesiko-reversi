## ゴール

マージ済みのルール、runtime、対戦、リプレイ、UIを既存エントリポイントへ配線し、二つのブラウザから遊んで棋譜を再生できるハッカソン用デモを完成させる。

## Agent assignment contract

- change-id: `complete-demo-integration`
- branch: `feature/complete-demo-integration`
- 専有パス:
  - `server/main.go`
  - `web/index.html`
  - `web/main.nako3`
  - `tests/e2e/**`
  - `scripts/demo-*.sh`
  - `openspec/changes/complete-demo-integration/**`
- 変更禁止:
  - 各internal packageと`rules/game/**`の内部実装
  - `openspec/specs/**`
  - `.github/workflows/**`を含む共有設定ファイル
- 依存Issue: #3、#4、#5、#6（すべて`develop`へマージ済みであること）

## 受け入れ条件

- [ ] `make dev`だけで静的配信、API、WebSocketを起動できる
- [ ] 二つのブラウザがマッチし、交互に着手して同じ確定盤面を見る
- [ ] 不正手は盤面を変えず、日本語エラーが表示される
- [ ] 終了時に勝者と平均色が両クライアントへ表示される
- [ ] 完了した対局のなでしこ棋譜を表示し、最初から再生できる
- [ ] `/healthz`がGoとgonakoのready状態を返す
- [ ] 正常系1本と不正手1本のE2Eテストがある
- [ ] 初見のPCでREADMEどおりに起動できる
- [ ] OpenSpecの全artifactが揃い、WSLで`make check`が成功する

## マージ条件

依存Issueのマージ後に開始する。feature PRではarchiveせず、マージ後にコーディネータが#1〜#7のchangeを依存順でarchiveする。
