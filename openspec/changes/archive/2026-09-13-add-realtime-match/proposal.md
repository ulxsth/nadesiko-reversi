## Why

契約、ルールengine、gonako adapterが揃い、1台のPCで交互に遊ぶローカルMVPも動くようになった。しかし現在の`localgame`は対局を1つしか持たず、二人を結びつける仕組みも、確定stateを両者へ配信する仕組みもない。

#22でtransportの順序と配信の契約が固まったので、それに沿った対戦セッションを実装する。#7がWebSocketへ配線できるよう、transportに依存しない境界として作る。

## What Changes

- 先着二人を1つのroomへ割り当て、待機と成立を配信する。
- サーバー権威で手番と盤面を確定し、受理のたびにstate全体をroomの全接続へ配信する。
- 同一`expectedTurn`の2件目を`stale_turn`で拒否し、盤面を変更しない。
- `commandId`を重複排除に使わず、event相関とログ出力だけに使う。
- 順番違反、別room、非参加者、席違いのcommandを拒否する。
- 切断でroomを再接続待ちへ移し、全員が切断したroomを破棄する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

なし。既存の`runtime-protocol`の順序・配信要件を実装するため、spec deltaは作成しない。

## Impact

- GitHub Issue: #5
- Branch: `feature/add-realtime-match`
- Dependencies: #1、#3、#22
- Exclusively owned paths: `server/internal/match/**`、`openspec/changes/add-realtime-match/**`

## Non-goals

- HTTP routeやWebSocketへの登録は行わない。`server/main.go`は#7が配線する。
- 永続化、棋譜の保存、リモートのマッチメイキングは扱わない。
- ルールの判断を複製しない。評価は#3の`runtime.Runner`だけを通す。
- `server/internal/runtime/**`、`server/internal/protocol/**`、`web/**`、`rules/**`は変更しない。
