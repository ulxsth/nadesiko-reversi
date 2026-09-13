# 対局記録の再生ハーネス

`harness.nako3`は、`docs/contracts/game-record.md`が定める棋譜の許可語彙を定義します。棋譜そのものはこの語彙だけで構成され、ハーネスと連結してgonakoで実行すると、読み取り結果と各手の盤面をJSON 1件で返します。

読み取りも再生も同じ1プロセスの中で完結します。棋譜1本あたりgonakoの起動はちょうど1回で、手数に比例して増えません。

```sh
cat rules/replay/harness.nako3 rules/replay/sample-game.nako3 > /tmp/record.nako3
echo 対局データ出力 >> /tmp/record.nako3
.tools/bin/gonako /tmp/record.nako3
```

上のように手で連結する場合、ハーネスの取り込み行は相対パスのままなので`/tmp`では解決できません。`rules/replay/`の下へ置くか、取り込み行を絶対パスへ書き換えてください。Goから呼ぶ場合は`server/internal/replay`の`LoadHarness`が絶対パスへ直します。

棋譜の読み取りはパースではなく実行で行うため、文法に合わない行はgonakoが行番号つきで落とします。

## ルールの取り込み

```nako3
!「../game/rules.nako3」を取り込む
```

`新規ゲーム作成`・`着手適用`・`パス適用`はここから来ます。ルールの判断をハーネスへ複製しないので、CLIと再生は必ず同じ結果になります。

## 語彙

| 語 | 例 |
| --- | --- |
| `ルール版宣言` | `「1」でルール版宣言` |
| `対局開始` | `「demo-1」と1で対局開始` |
| `黒着手` | `2と2で黒着手` |
| `白着手` | `2と3で白着手` |
| `黒パス` | `黒パス` |
| `白パス` | `白パス` |
| `対局終了` | `「白」で対局終了` |

手番語の`黒`は契約の`dark`、`白`は`light`に対応します。行と列は0から7で、盤の左上が0行0列です。

この表にない行は`server/internal/replay`の`Scan`が実行前に拒否します。棋譜はgonakoで実行するため、許可語彙を素通しにすると任意コード実行になるからです。

## 注釈

`#`以降は再生時に無視されます。着手行には置いた駒の色と変換された駒数、終了行には総手数・色合計・駒数を書きます。どれも盤面から決定的に導かれる値なので、厳密照合モードでは再生結果と突き合わせて棋譜の破損を検出します。

## サンプル

`sample-game.nako3`は実際のルールengineで最後まで進めた60手の対局です。`server/internal/replay`のtestが再生して、記録された勝者と注釈に一致することを確認します。作り直すには次を実行します。

```sh
UPDATE_SAMPLE=1 .tools/go/bin/go test ./server/internal/replay/ -run TestSampleRecordReplays
```
