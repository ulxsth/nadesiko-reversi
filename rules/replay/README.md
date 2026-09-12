# 対局記録の再生ハーネス

`harness.nako3`は、`docs/contracts/game-record.md`が定める棋譜の6語を定義します。棋譜そのものはこの6語だけで構成され、ハーネスと連結してgonakoで実行すると、対局データをJSON 1件で返します。

```sh
cat rules/replay/harness.nako3 rules/replay/sample-game.nako3 > /tmp/record.nako3
echo 対局データ出力 >> /tmp/record.nako3
.tools/bin/gonako /tmp/record.nako3
```

Goからは`server/internal/replay`が同じ連結を行います。棋譜の読み取りはパースではなく実行で行うため、文法に合わない行はgonakoが行番号つきで落とします。

## 語彙

| 語 | 例 |
| --- | --- |
| `対局開始` | `「demo-1」と1で対局開始` |
| `黒着手` | `2と2で黒着手` |
| `白着手` | `2と3で白着手` |
| `黒パス` | `黒パス` |
| `白パス` | `白パス` |
| `対局終了` | `「白」で対局終了` |

手番語の`黒`は契約の`dark`、`白`は`light`に対応します。行と列は0から7で、盤の左上が0行0列です。

## 注釈

`#`以降は再生時に無視されます。着手行には置いた駒の色と変換された駒数、終了行には総手数・色合計・駒数を書きます。どれも盤面から決定的に導かれる値なので、厳密照合モードでは再生結果と突き合わせて棋譜の破損を検出します。

## ルール適用について

このハーネスはルールを適用しません。`rules/game/main.nako3`は末尾で無条件に`CLI実行`するため取り込めず、`新規ゲーム作成`・`着手適用`・`パス適用`へ委譲できないからです。Issue #6のブロッカーとして記録済みで、取り込みガードが入るまでは`server/internal/replay`が同じルールengineへ再適用します。

## サンプル

`sample-game.nako3`は実際のルールengineで最後まで進めた60手の対局です。`server/internal/replay`のtestが再生して、記録された勝者と注釈に一致することを確認します。作り直すには次を実行します。

```sh
UPDATE_SAMPLE=1 .tools/go/bin/go test ./server/internal/replay/ -run TestSampleRecordReplays
```
