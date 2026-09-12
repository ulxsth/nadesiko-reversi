# 実runtime E2Eを手動デバッグへ置き換える

## Why

ローカルMVPのblack-box E2Eは、テストごとにGo serverとgonako processを起動し、対局終了まで多数のcommandを評価する。実装横断の回帰検知には有効だが、常時実行すると開発待ち時間と保守範囲が大きい。

## What Changes

- `tests/e2e`の実runtimeテストを削除し、`make check`をpackage test中心へ戻す。
- 実server、gonako、ブラウザの結合確認を、人間が再現できるチェックリストとして文書化する。
- 後続の統合Issue #7とアーキテクチャ文書を、同じ検証方針へ更新する。

## 非ゴール

- 既存のルール、protocol、runtime、localgame package testは削除しない。
- CI、Makefile、production code、UIは変更しない。
- 完了済みOpenSpec archiveは書き換えない。

## 所有範囲

- `tests/e2e/**`
- `docs/manual-debug.md`
- `docs/architecture.md`
- `docs/p0-backlog.md`
- `.github/issue-bodies/p0-07-integration.md`
- `openspec/changes/replace-e2e-with-manual-debug/**`
