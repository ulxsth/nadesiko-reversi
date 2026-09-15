# Agent向け作業規約

このリポジトリでは、複数Agentの変更を競合させずに`develop`へ集約します。

## 作業を始める前に

1. 担当Issueの`change-id`、依存Issue、専有パスを確認する。
2. 依存Issueがすべて`develop`へマージ済みであることを確認する。
3. 最新の`origin/develop`から`feature/<change-id>`を作る。
4. OpenSpec changeはIssueと同じ`change-id`を使う。

## Worktree運用

- 実装AgentはIssueごとに専用worktreeを使用する。
- 1 Issue＝1 `change-id`＝1 branch＝1 worktree＝1 Agentとする。
- worktreeとbranchは最新の`origin/develop`から作る。
- 複数Agentが同じcheckoutやworktreeを共有してはならない。
- primary checkoutは、Issue整理、PRマージ、OpenSpec archiveなどコーディネータ作業専用とする。
- 実装Agentは専用worktree内でのみ編集、テスト、commit、pushを行う。
- 別worktreeでcheckout済みのbranchを移動、削除、再利用してはならない。
- GitリポジトリにCodex taskを作る場合は、原則としてworktree環境を選ぶ。
- `.tools`が存在しないworktreeでは、テスト前に`make bootstrap`を実行する。
- PRマージ後、worktreeがcleanであることを確認してから削除する。
- 未commit変更があるworktreeを`--force`で削除してはならない。

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
- 完了済みchangeは差分specを本体specへ同期・検証してからarchiveする。この通常フローは都度確認せず進め、同期できない場合はarchiveせず報告する。

## 完了条件

- `make check`がWSLで成功する。
- Issueの受け入れ条件をPR本文で一つずつ確認する。
- PRのbaseは`develop`にする。`main`へ直接PRしない。
- 担当外の整形、リネーム、依存更新を混ぜない。

## PRマージ後のIssue整理

実装Agentはfeature PRの本文へ`Closes #<Issue番号>`と後続Issueへの影響を書く。feature PRのbaseは既定ブランチではなく`develop`なので、マージ担当者は自動クローズだけに依存せず、マージ直後に次を行う。

1. 担当IssueがCLOSEDになったことを確認する。OPENのままなら、マージPRをコメントして手動で閉じる。
2. 担当Issueを依存先に持つOPEN Issueを確認する。
3. すべての依存Issueが`develop`へマージ済みなら、`blocked`を外して`agent-ready`を付ける。
4. 未完了の追加依存がある場合は、`agent-ready`を外して`blocked`を付け、依存Issue番号をコメントする。
5. `human-task`のIssueには`agent-ready`を付けない。
6. OpenSpec archive PRのマージでは、実装Issueの状態や依存ラベルを再変更しない。

Issueのクローズと依存ラベルの更新はマージ担当者の責務とする。
