## Why

ルール、ローカル盤面、対戦セッション、棋譜再生は個別に実装済みだが、現行の起動経路は1ブラウザの交代制対局に留まり、二人が別ブラウザから遊んで棋譜を見るデモになっていない。#7 で既存部品を同一プロセスの入口へ接続し、ハッカソンで手元から起動・説明できる状態にする。

## What Changes

- `make dev` のGoサーバーに二人対戦用WebSocket、確定対局の棋譜・再生取得、Go/gonakoのready表示を接続する。
- ブラウザに二人対戦への参加、席と接続状態、確定stateの同期、送信者だけへの日本語エラー、終局時の勝者・平均色、棋譜表示と初期局面からの再生を追加する。
- 既存のローカル交代制対局とそのAPIを残し、二人対戦は明示的な参加操作で切り替える。ルール判定はサーバー側の既存packageだけに委ねる。
- 人が実施する二画面の正常系・不正手・終局・再生の手順を更新し、実施結果を統合PRに記録する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `browser-game-ui`: 既存のローカル操作を保ちながら、別ブラウザ同士の対戦、接続状態、終局の平均色、棋譜表示と再生の操作を追加する。
- `runtime-protocol`: WebSocketの参加・command・配信、対局終了後の棋譜取得・再生、Go/gonakoのready確認をブラウザから使える契約として追加する。

## Impact

- GitHub Issue: #7。依存: #3、#4、#5、#6、#30、#35（すべて`develop`へマージ済み）。ブランチ: `feature/complete-demo-integration`。
- 専有パス: `server/main.go`、`web/index.html`、`web/main.nako3`、`docs/manual-debug.md`、`scripts/demo-*.sh`、`openspec/changes/complete-demo-integration/**`。
- `server/internal/{runtime,match,replay,protocol}/**`と`rules/game/**`の公開APIを利用するが、その内部や`openspec/specs/**`は変更しない。`README.md`、`go.mod`、`Makefile`、`web/styles.css`、共有CI設定も変更しない。

## Non-goals

- AWSへのデプロイ、永続DB、認証、遠隔対戦、サーバー再起動後の対局復旧。
- ルール・勝敗基準・JSON契約v1の意味の変更、ブラウザ自動E2Eの追加、READMEの編集。
