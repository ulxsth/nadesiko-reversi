## Why

#5のローカル二人対戦と、その先のリモート対戦へ進む前に、transportの順序制御を確定する必要がある。現契約はroom単位の`sequence`でeventの重複と欠落を検出する設計になっている。しかし本ゲームはターン制で、受理したcommand 1件ごとに`turnNumber`がちょうど1増える。この性質のため`sequence`はゲームeventだけを数える限り常に`turnNumber`と同値であり、重複排除も`expectedTurn`だけで満たせる。

二重の機構を残したまま実装へ進むと、#5と#7が同じ不変条件を別々の場所で担保することになり、ずれたときの原因追跡が難しくなる。実装が始まる前に契約側で一本化する。

## What Changes

- transportから`sequence`を削除し、event順序の識別を`turnNumber`へ一本化する。
- 重複commandと再送の扱いを`expectedTurn`による`stale_turn`拒否へ一本化し、`commandId`を重複排除の鍵として使うことを禁止する。
- 成功時にstateの差分ではなくgame state全体を配信することを契約とし、欠落検出機構を不要にする。
- `commandId`の用途をeventとの相関およびログ出力へ限定する。
- クライアントが`stale_turn`をエラー表示せず再同期として扱うことを定める。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `runtime-protocol`: `transport eventの順序`を削除し、`expectedTurnによるcommand順序と冪等性`と`成功時の全state配信`を追加する。`command表現`へ`commandId`の用途限定を加える。

## Impact

- GitHub Issue: #22
- Branch: `feature/simplify-transport-ordering`
- Dependencies: なし
- Exclusively owned paths: `docs/contracts/protocol.md`、`openspec/changes/simplify-transport-ordering/**`
- #5は受け入れ条件「重複commandを拒否する」を、同一`expectedTurn`の2件目を`stale_turn`で拒否する形で満たす。
- #7は`sequence`の配線を実装しない。

## Non-goals

- `rngState`の秘匿と、席の割り当てによるplayer詐称防止は扱わない。ハッカソン範囲外としてトリアージ済み。
- WebSocketのメッセージ種別、handler、room管理の実装は行わない。#5と#7が担当する。
- `common.schema.json`の`commandId`必須指定は変更しない。用途の限定であってfieldの削除ではない。
- `openspec/specs/**`は編集しない。コーディネータがマージ後のarchiveで反映する。
