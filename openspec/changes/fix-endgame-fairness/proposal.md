## Why

終局の公平性に影響する欠陥が2件ある。どちらも`rules/game/rules.nako3`にあり、どちらも盤面の互換性を壊すため、1つのchangeで扱ってルール版を1回だけ上げる。

1. **127.5ちょうどの終局が`light`の勝ちになる。** `勝者計算`は`2*色合計 < 駒数*255`なら`dark`、それ以外は`light`を返す。平均がちょうど127.5の終局は設計判断ではなく、元実装の二分岐の`else`側へ落ちた結果にすぎない。このゲームの勝敗は「盤面全体の色の重心が127.5のどちら側にあるか」という連続量の比較であり、ちょうど中央はどちらのものでもない。
2. **色変換の切り捨てが`dark`へ偏る。** 変換色は`floor((a1+a2+b1)/3)`で常に切り捨てるため、1変換あたり平均1/3の色が下（＝黒側）へ漏れる。`rules/replay/sample-game.nako3`の実測では60手で延べ354駒が変換され、期待損失は約118色単位になる。判定閾値8160に対して大差の局では誤差だが、接戦では決定的に効く。

## What Changes

- 色の重心がちょうど127.5になる終局を引き分けとし、`phase`=`finished`かつ`winner`=`null`で表す。
- 色変換を`round`へ変える。浮動小数を使わず`floor((2S+3)/6)`（`S=a1+a2+b1`）で計算する。
- 棋譜の終了行に手番語`引分`を足し、引き分けを書き出し・再生できるようにする。
- `rulesVersion`を`2`へ上げ、版`1`の棋譜を`record_unsupported_rules_version`で拒否する。
- 盤面UIが引き分けを表示できるようにする。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `gradient-reversi-rules`: 色変換の丸めと、終了時の勝敗判定。
- `runtime-protocol`: `finished`時の`winner`のnull許容、対局記録の勝者表現、棋譜の許可語彙。
- `browser-game-ui`: 終局結果ポップアップの引き分け表示。

## Impact

- GitHub Issue: #40
- Branch: `feature/fix-endgame-fairness`
- Dependencies: #30（`develop`へマージ済み）
- Exclusively owned paths:
  - `docs/contracts/game-rules.md`
  - `docs/contracts/game-state.schema.json`
  - `docs/contracts/game-record.md`
  - `docs/contracts/game-record.schema.json`
  - `docs/contracts/source-differences.md`
  - `rules/game/**`
  - `rules/replay/**`
  - `server/internal/protocol/**`
  - `server/internal/replay/**`
  - `web/**`（`vendor`を除く）
  - `openspec/changes/fix-endgame-fairness/**`

この変更で元実装との盤面互換性は完全に失われる。同じseedから別の盤面になり、版`1`の既存棋譜は再生できなくなる。#30で入れた版宣言の仕組みが最初に働く案件になる。

## Non-goals

- 引き分けを避けるための二次基準は導入しない。単一の評価軸（色の重心）へ別の軸を混ぜるとルールの説明が複雑になり、二次基準でも同点になり得るため、結局引き分けの表現が必要になる。
- 誤差の繰り越しや剰余の分配は採らない。「走査順に依存しない結果を返す」既存の要件と衝突する。
- 色を高精度の内部表現へ変えない。`board`が0〜255の整数という契約の根幹を変えるためコストが見合わない。
- 版`1`の棋譜を版`2`へ変換する移行機能は作らない。明示的に拒否するだけにする。
- `server/main.go`、`server/internal/runtime/**`、`server/internal/match/**`、`openspec/specs/**`、共有設定ファイルは変更しない。
