## Why

`gradient-reversi-rules`の仕様は確定したが、現在の`rules/`にはgonako起動確認しかなく、ゲーム状態を評価できない。ローカルプレイMVPの正本となるルールエンジンをなでしこで実装する。

## What Changes

- JSON requestから新規ゲームを生成する。
- 完全なgame stateへplace/pass commandを原子的に適用する。
- 合法手、8方向の挟み、色変換、手番、終了、勝敗をなでしこで計算する。
- 固定vectorを使うgonako単体テストを追加する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

なし。既存の`gradient-reversi-rules`と`runtime-protocol`を実装するため、spec deltaは作成しない。

## Impact

- GitHub Issue: #2
- Branch: `feature/implement-game-rules`
- Dependencies: #1
- Exclusively owned paths: `rules/game/**`, `rules/testdata/**`, `openspec/changes/implement-game-rules/**`

## Non-goals

- Goからのprocess呼び出し、HTTP API、WebSocket、ブラウザUIは実装しない。
- `server/**`、`web/**`、main specは変更しない。
