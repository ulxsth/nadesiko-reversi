# Agent向け作業規約

このリポジトリでは、複数Agentの変更を競合させずに`develop`へ集約します。

## 作業を始める前に

1. 担当Issueの`change-id`、依存Issue、専有パスを確認する。
2. 依存Issueがすべて`develop`へマージ済みであることを確認する。
3. 最新の`origin/develop`から`feature/<change-id>`を作る。
4. OpenSpec changeはIssueと同じ`change-id`を使う。

## 編集範囲

- README.md は人間が編集するので手を付けない。
- 担当Issueに列挙された専有パスだけを編集する。
- 別の進行中Issueが専有するパスを編集しない。
- `go.mod`、`Makefile`、`openspec/config.yaml`、`.github/workflows/**`、`server/main.go`など共有ファイルが必要になった場合は、勝手に変更せずIssueへブロッカーとして記録する。
- `.agents/skills/openspec-*/SKILL.md`はOpenSpec CLIの生成物なので直接編集しない。
- `openspec/specs/**`はコーディネータの単一書き込み領域とする。

## OpenSpec

- 計画: `$openspec-propose <change-id>`
- 実装: `$openspec-apply-change <change-id>`
- feature PRには`openspec/changes/<change-id>/**`を含める。
- featureブランチではarchiveしない。
- archiveは`develop`へのマージ後にコーディネータが直列実行する。

## 完了条件

- `make check`がWSLで成功する。
- Issueの受け入れ条件をPR本文で一つずつ確認する。
- PRのbaseは`develop`にする。`main`へ直接PRしない。
- 担当外の整形、リネーム、依存更新を混ぜない。
