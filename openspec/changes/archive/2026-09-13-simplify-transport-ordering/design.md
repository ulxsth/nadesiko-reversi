## Context

動機は`proposal.md`を参照。現在の`runtime-protocol`は、transport層に`sequence`という独立した連番を置き、クライアントが重複と欠落を検出する設計になっている。これは一般のリアルタイム配信では妥当だが、本ゲームには二つの前提がある。

一つは`turnNumber`の定義で、`docs/contracts/protocol.md`が「受理済みcommand数」としている。受理のたびにちょうど1増えるため、ゲームeventだけを数える`sequence`と値が常に一致する。もう一つは`expectedTurn`と拒否時の原子性で、`gradient-reversi-rules`が拒否時に`rngState`を含む全fieldを変更しないと定めている。この二つが揃うと、順序・重複・欠落の制御に必要な情報は`turnNumber`だけで足りる。

## Goals / Non-Goals

**Goals:**

- 順序制御の担保箇所を1か所へ寄せ、#5と#7が同じ不変条件を二重に実装しないようにする。
- at-least-onceな再送をクライアントが安全に行える根拠を契約として明示する。
- Issue #22の専有パス`docs/contracts/protocol.md`と`openspec/changes/simplify-transport-ordering/**`だけで完結させる。

**Non-Goals:**

- transportの実装、room管理、WebSocketのmessage種別の決定。
- 敵対的クライアントへの対策（`rngState`秘匿、席の束縛）。
- ルールengineとJSON Schemaの変更。

## Decisions

### sequenceを削除しturnNumberへ一本化する

`sequence`がゲームeventだけを数える限り`turnNumber`と同値になるため、二つ持つ意味がない。`sequence`が独自の値を持つのは、着席通知や切断通知のような非ゲームeventを同じ順序線へ乗せる場合だけだが、P0の範囲でその要求はない。将来それが必要になった時点で、非ゲームeventを含む順序要件として改めて追加する。

### 重複と再送はexpectedTurnだけで扱う

受理のたびに`turnNumber`が1増えるため、同じ`expectedTurn`の2件目は必ず`stale_turn`になる。ダブルクリックによる重複も、応答消失後の再送も、同じ経路で正しく処理される。適用済みなら`stale_turn`、未適用なら受理という分岐が自動的に成立するため、`commandId`を鍵にした重複排除表を持つ必要がない。

厳密には、同じ`expectedTurn`で別内容のcommandを送れば2件目は拒否される。これは「1手番につき1手」というゲームの制約そのものなので、望ましい挙動として受け入れる。

### 成功時はstate全体を配信する

盤面は64マスで、state全体でも1KB程度にしかならない。差分配信の帯域上の利点がないため、毎回全体を送る。クライアントが差分を積む必要がなくなり、取りこぼしても次の1通で追いつくので、欠落検出そのものが不要になる。再接続も同じ経路で扱えるため、専用の再同期requestを定義しない。

### commandIdは相関IDとして残す

`commandId`をfieldごと削除すると、eventがどのcommandへの応答かをクライアントが対応づけられなくなり、ログの追跡も難しくなる。用途を相関とログへ限定した上でfieldは残す。`common.schema.json`は変更しない。

### stale_turnを無害な再同期として扱う

`stale_turn`は正常な競合の結果として日常的に発生する。クライアントがこれを赤いエラー表示にすると、ダブルクリックのたびに画面が汚れる。応答に含まれる現在stateで描き直して黙る、という挙動を契約側で定める。

## 専有パスとの関係

この変更が触るのは`docs/contracts/protocol.md`のTransport節だけで、schemaファイル、`rules/**`、`server/**`、`web/**`には手を入れない。spec deltaは`openspec/changes/simplify-transport-ordering/specs/runtime-protocol/spec.md`に置き、`openspec/specs/**`はコーディネータのarchiveに任せる。

## 統合点

- #5 `add-realtime-match`: 受け入れ条件「順番違反、別room、不正手、重複commandを拒否する」のうち重複commandは、同一`expectedTurn`の2件目を`stale_turn`で拒否する形で満たす。本changeでは#5のファイルを編集しない。
- #7 `complete-demo-integration`: `sequence`の配線を実装しない。
- #6 `add-executable-replay`: 棋譜は手行の並び順が`turnNumber`に対応するため、`expectedTurn`と`commandId`を記録しない。

## Risks / Trade-offs

- [非ゲームeventを同じ順序線へ乗せたくなったとき`sequence`が必要になる] → その時点で非ゲームeventを含む順序要件として追加する。P0では発生しない。
- [state全体配信の帯域が対局数に比例して増える] → 1配信あたり約1KBかつターン制で人間の操作間隔があるため、P0の規模では問題にならない。
- [`commandId`が残ることで重複排除に使われる実装が現れる] → specへMUST NOTとして明記し、#5のreviewで確認する。

## Migration Plan

`sequence`はまだどの実装にも存在しないため、移行対象のコードとデータはない。#22のマージ後にコーディネータがarchiveし、`openspec/specs/runtime-protocol/spec.md`へ反映する。#5はarchive後のmain specを参照して着手する。
