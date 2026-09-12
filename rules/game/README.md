# なでしこルールエンジン

ゲームルールの正本は`rules.nako3`に閉じています。`main.nako3`はそれを取り込む一回起動型CLIで、JSON requestを標準入力から1件受け取り、JSON responseを標準出力へ1件返します。

```sh
.tools/bin/gonako rules/game/main.nako3 < rules/testdata/new-game.json
.tools/bin/gonako rules/game/main.nako3 --test
```

通常起動のstdoutは機械可読JSONだけです。壊れたJSONはgonakoが非0で終了し、診断を出力します。

## ファイルの分担

| ファイル | 中身 |
| --- | --- |
| `rules.nako3` | 盤面、合法手、色変換、乱数、終了と勝敗、`要求評価`。関数定義と定数だけで、トップレベルの入出力を持たない |
| `main.nako3` | `rules.nako3`の取り込み、`自己テスト`、`CLI実行`、引数の振り分け |

分けてあるのは、棋譜の再生ハーネスからルールだけを取り込めるようにするためです。`main.nako3`は末尾で必ずCLIを実行するので、そのままでは取り込めません。

```nako3
!「rules.nako3」を取り込む
```

`rules.nako3`は単体で実行しても何も出力しません。`server/internal/replay`のtestがこれを常に確認します。
