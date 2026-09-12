## Why

`gradient-reversi-rules`と`runtime-protocol`の仕様は確定し、#2でなでしこ製ルールengineも動くようになった。しかしGo側には契約のJSONを表す型がなく、gonakoを呼ぶ手段もない。#5、#6、#7がそれぞれ独自にprocess呼び出しを書くと、timeoutやerror変換の方針が分散する。ルール評価の境界を1か所へ閉じ、後続Issueが共有できる形にする。

## What Changes

- `runtime-protocol`のrequest、response、game state、command、eventをGoの型として表現し、契約のvalidationを付ける。
- gonakoの実行ファイル、ルールsource、stdin/stdoutをruntime adapterへ閉じ込める。
- timeout、終了コード、stderr、解釈できない出力を型付きerrorへ変換する。
- handlerとmatch serviceが実processなしで試せる差し替え実装を提供する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

なし。既存の`runtime-protocol`をGo側で実装するため、spec deltaは作成しない。

## Impact

- GitHub Issue: #3
- Branch: `feature/integrate-gonako-runtime`
- Dependencies: #1、#2
- Exclusively owned paths: `server/internal/runtime/**`、`server/internal/protocol/**`、`openspec/changes/integrate-gonako-runtime/**`

## Non-goals

- HTTP routeやWebSocketへの登録は行わない。`server/main.go`は#7と#10が配線する。
- マッチメイキング、対局の保持、棋譜の保存は扱わない。
- ルールの判断をGoへ複製しない。合法手も色変換も勝敗もgonako側の結果をそのまま扱う。
- `rules/**`、`web/**`、main specは変更しない。
