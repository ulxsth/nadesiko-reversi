## Why

`runtime-protocol`でgame stateの形は決まったが、ブラウザ側にはstateを描画する手段がない。現在の`web/main.nako3`は盤を描いてクリック位置に適当な色を置くだけの縦切りで、契約のstateもcommandも扱っていない。#5と#7が配線する前に、stateを描いてplayerの着手意図をcommandへ変換するデモ品質のUIを用意する。

## What Changes

- 契約のgame state JSONから8×8盤面と0〜255の駒色を描画する。
- 手番、相手待ち、終了、エラーを日本語で表示する。
- 次の駒色、合法手、選択中のマスを視覚的に区別する。
- クリック・タップ・キーボードを契約のcommandへ変換する。
- 確定応答を受け取るまで盤面を更新しない送信境界を作る。
- 通信なしで主要状態を確認できるfixtureを追加する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

なし。既存の`runtime-protocol`と`gradient-reversi-rules`をブラウザ側で描画するため、spec deltaは作成しない。

## Impact

- GitHub Issue: #4
- Branch: `feature/build-board-client`
- Dependencies: #1
- Exclusively owned paths: `web/index.html`、`web/main.nako3`、`web/styles.css`、`web/fixtures/**`、`openspec/changes/build-board-client/**`

## Non-goals

- WebSocketやHTTPの実接続は配線しない。送信関数の中身は#7が実装する。
- ルール判断をブラウザへ複製しない。合法手はstateの`legalMoves`だけを見る。
- マッチメイキング、棋譜再生UI、観戦は扱わない。
- `web/vendor/**`、`server/**`、`rules/**`、main specは変更しない。
