# なでしこルールエンジン

JSON requestを標準入力から1件受け取り、JSON responseを標準出力へ1件返す一回起動型CLIです。ゲームルールの正本は`main.nako3`に閉じています。

```sh
.tools/bin/gonako rules/game/main.nako3 < rules/testdata/new-game.json
.tools/bin/gonako rules/game/main.nako3 --test
```

通常起動のstdoutは機械可読JSONだけです。壊れたJSONはgonakoが非0で終了し、診断を出力します。
