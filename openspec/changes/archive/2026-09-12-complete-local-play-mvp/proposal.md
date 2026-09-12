## Why

なでしこ製ルール、Go-gonako adapter、ブラウザ盤面は個別に動くが、まだ同じ対局として接続されていない。ハッカソンMVPとして、`make dev`から1ブラウザで二人が最後まで交互に遊べる縦切りを完成させる。

## What Changes

- Goサーバーに、1対局を保持するローカルruntime endpointを追加する。
- ブラウザの送信境界をendpointへ接続し、newGame、place、pass、resetを実行する。
- サーバーを起動するblack-box E2Eで正常着手、不正着手、seed再現、対局終了を検証する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

なし。既存の`gradient-reversi-rules`と`runtime-protocol`をローカルtransportへ配線するため、spec deltaは作成しない。

## Impact

- GitHub Issue: #10
- Branch: `feature/complete-local-play-mvp`
- Dependencies: #2、#3、#4
- Exclusively owned paths: `server/main.go`、`server/internal/localgame/**`、`web/index.html`、`web/main.nako3`、`tests/e2e/**`、`openspec/changes/complete-local-play-mvp/**`

## Non-goals

- WebSocket、複数room、matchmaking、永続化、network対戦は実装しない。
- ゲームルールをGoまたはブラウザへ複製しない。
- 見た目の追加調整は別Issueに残す。
