## Context

`openspec/specs/gradient-reversi-rules/spec.md`と`openspec/specs/runtime-protocol/spec.md`が外部挙動の正本である。gonako 3.8.4には`標準入力全取得`、`JSONデコード`、`JSONエンコード`、`ASSERT等`がある。

## Goals / Non-Goals

**Goals:**

- JSON object 1件を標準入力から読み、JSON object 1件だけを標準出力へ返す。
- ルールロジックを`rules/game/**`内に閉じ、#3からprocessとして呼べるようにする。
- seed=1、盤端、複数方向、不正command、パス、勝敗の固定vectorを実行する。

**Non-Goals:**

- transport、並行実行制御、永続化。
- 通常のリバーシへのルール変更。

## Decisions

### 一回起動・一回評価のCLIにする

`rules/game/main.nako3`はstdinを最後まで読み、requestを1件評価し、responseを1行で出力する。常駐processより分離性が高く、#3でtimeoutとstderrを扱いやすい。

### 状態は入力を直接変更せず複製してから評価する

拒否時の原子性を守るため、validation完了前に入力stateを変更しない。成功時だけ複製stateをresponseへ採用する。

### 各方向を着手前盤面から評価する

8方向のrayは対象マス以外で交差しないため方向ごとの変換列を保持し、各計算は着手前盤面を参照する。合法手はrow-major走査で安定順序にする。

### 自己テストもなでしこで書く

`rules/game/main.nako3 --test`でgonakoの`ASSERT等`を使う固定vectorを検証する。通常起動ではstdin CLIを実行し、テスト時だけ自己テストへ分岐する。

## Risks / Trade-offs

- [なでしこの数値型でLCG計算に丸めが入る] → 今回の最大積が2^53未満であることを確認し、固定vectorで各遷移を検証する。
- [取り込み時にCLI本体まで実行される] → `--test`引数を検出し、test runnerから関数だけを利用する。
- [意味制約をJSON decodeだけで検証できない] → stateの全必須fieldと64マスを明示検証する。
