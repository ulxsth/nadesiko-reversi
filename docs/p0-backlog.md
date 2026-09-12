# P0バックログ

各行は「Issue 1件＝OpenSpec change 1件＝featureブランチ1本＝担当者1人」です。依存Issueが未マージの作業は開始しません。

| Issue | change-id | 専有領域 | 依存 |
| --- | --- | --- | --- |
| #1 ゲーム・通信契約 | `define-game-contract` | `docs/contracts/**` | なし |
| #2 なでしこ製ルールエンジン | `implement-game-rules` | `rules/game/**`, `rules/testdata/**` | #1 |
| #3 gonako実行アダプター | `integrate-gonako-runtime` | `server/internal/runtime/**`, `server/internal/protocol/**` | #1, #2 |
| #4 なでしこ3盤面UI | `build-board-client` | `web/**`（`vendor`除外） | #1 |
| #5 ローカル二人対戦 | `add-realtime-match` | `server/internal/match/**` | #1, #3 |
| #6 対局ログ保存・再生 | `add-executable-replay` | `server/internal/replay/**`, `rules/replay/**` | #1, #2, #3 |
| #7 統合とデモ導線 | `complete-demo-integration` | `server/main.go`, `docs/manual-debug.md`, 統合時に必要な既存UIファイル | #3, #4, #5, #6 |

すべてのIssueは、自身の`openspec/changes/<change-id>/**`も専有します。`openspec/specs/**`、`.agents/**`、`openspec/config.yaml`、`go.mod`、`Makefile`、`.github/workflows/**`はコーディネータ専有です。

クリティカルパスは `#1 → #2 → #3 → #5 → #7` です。#4は#2と並行でき、#6は#3の後に#5と並行できます。
