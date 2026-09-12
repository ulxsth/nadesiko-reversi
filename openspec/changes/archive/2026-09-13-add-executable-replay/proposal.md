## Why

契約とルールengineとgonako adapterが揃い、ローカル対戦も動くようになったが、終わった対局は消えたままになる。元実装はWhitespaceへ変換して棋譜を保存していたが、これは移植しない。代わりに「日本語コードそのものが棋譜」という見せ場へ置き換える。

`docs/contracts/game-record.md`が棋譜の文法・語彙・注釈・error codeを定めたので、それを実装する。

## What Changes

- 確定した対局を、そのまま実行できるなでしこコードへ直列化する。
- 棋譜をgonakoで実行して読み取り、各手または指定手数の盤面を復元する。
- 記録された勝者と注釈の導出値が再生結果と一致することを確認する。
- 不正・改ざん棋譜を、行番号とその行のソースを添えて拒否する。
- 保存・一覧・ID取得・ランダム1件取得をserviceとして公開する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

なし。`docs/contracts/game-record.md`の実装であり、main specへのdeltaは作成しない。

## Impact

- GitHub Issue: #6
- Branch: `feature/add-executable-replay`
- Dependencies: #1、#2、#3
- Exclusively owned paths: `server/internal/replay/**`、`rules/replay/**`、`openspec/changes/add-executable-replay/**`

## Non-goals

- UIとrouteへの配線は行わない。`server/main.go`と`web/**`は#7が扱う。
- `records/<gameId>.nako3`への永続化は行わない。P0はプロセス内保存で足り、Store interfaceで差し替えられるようにする。
- ルールの判断を複製しない。再生は#3のruntime adapter経由で同じルールengineへ適用する。

## ブロッカー

契約は再生ハーネスが`rules/game/main.nako3`の`新規ゲーム作成`・`着手適用`・`パス適用`へ委譲する形を想定している。しかし`main.nako3`は末尾で引数の有無にかかわらず`CLI実行`するため、取り込むと標準入力を読みに行って落ちる。`rules/game/**`はこのIssueの変更禁止パスなので、取り込みガードは入れていない。

暫定として、ハーネスは棋譜の読み取りだけを担い、ルール適用は`server/internal/replay`が#3のinterface経由で行う。同じルールengineを使うため再生結果は変わらない。ガードが入り次第、ハーネス内の委譲へ移せる。
