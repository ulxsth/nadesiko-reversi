# Runtime protocol v1

ブラウザ、Go、gonakoの境界ではJSON object 1件を入力し、JSON object 1件を返す。すべての主要objectは`version: "1"`を持つ。Schemaの正本は同じディレクトリの`*.schema.json`。

## Runtime request

### 新規ゲーム

```json
{
  "version": "1",
  "action": "newGame",
  "gameId": "local-demo",
  "seed": 1
}
```

### command適用

```json
{
  "version": "1",
  "action": "applyCommand",
  "state": {},
  "command": {
    "commandId": "cmd-1",
    "type": "place",
    "player": "dark",
    "expectedTurn": 0,
    "row": 2,
    "col": 2
  }
}
```

`pass` commandは`row`、`col`を持たない。

## Game state

| field | 型 | 意味 |
| --- | --- | --- |
| `version` | `"1"` | protocol version |
| `gameId` | string | 対局ID |
| `board` | `(null\|0..255)[64]` | row-major盤面 |
| `turnNumber` | 0以上の整数 | 受理済みcommand数 |
| `currentPlayer` | `dark\|light` | 現在の手番 |
| `nextColor` | 0..255 | 次に置く色 |
| `rngState` | 0..4294967295 | LCG内部state |
| `consecutivePasses` | 0..2 | 連続パス数 |
| `phase` | `playing\|finished` | 対局状態 |
| `winner` | `null\|dark\|light` | 終了時の勝者 |
| `legalMoves` | 座標配列 | row-major順の現在の合法手 |

## Runtime response

成功時は`ok: true`と更新後`state`を返す。command適用時の`event`には`commandId`、`turnNumber`、`player`、`type`を含め、着手なら`row`、`col`、`placedColor`、変更された駒の`changes`も含める。

失敗時は`ok: false`、`error`、変更前`state`を返す。`error.message`は表示可能な日本語、`error.code`は次の安定値を使う。

| code | 条件 |
| --- | --- |
| `invalid_json` | JSONとして解析できない |
| `unsupported_version` | version欠落または未対応 |
| `invalid_request` | actionに必要なfieldがない |
| `invalid_state` | stateの形または値が不正 |
| `invalid_command` | commandの形またはtypeが不正 |
| `invalid_coordinate` | row/colが0〜7の外 |
| `occupied` | 対象マスが埋まっている |
| `illegal_move` | 1方向も挟みが成立しない |
| `not_your_turn` | playerが現在手番と違う |
| `stale_turn` | expectedTurnが現在値と違う |
| `pass_not_allowed` | 合法手が存在する |
| `game_finished` | 終了後のcommand |

拒否時は乱数を含むstateを変更しない。壊れたJSONでresponseを作れない実行形態では、非0終了とstderr診断を許容する。

## Transport

ローカルHTTPはruntime responseをそのままbodyに使う。将来のWebSocketでは成功eventへroom単位の`sequence`を付け、成功ごとに1増やす。拒否commandではsequenceを進めない。

## Game record

recordはgame ID、seed、開始・終了時刻、勝者、受理済みcommand列、最終stateを保持する。seedからcommandを順に再適用した結果が最終stateと一致することを検証する。時刻はRFC 3339 UTC文字列とする。
