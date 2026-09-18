## 1. 契約の更新

- [x] 1.1 `docs/contracts/game-rules.md`の色変換を`round`の定義へ書き換え、丸めの対応表を載せる
- [x] 1.2 `docs/contracts/game-rules.md`の勝敗判定から「元実装互換で`light`」を消し、引き分けの定義へ置き換える
- [x] 1.3 `docs/contracts/game-state.schema.json`の`finished`分岐で`winner`にnullを許す
- [x] 1.4 `docs/contracts/game-record.md`の終了行の文法・許可語彙・手番語の対応表・ルール版を`2`へ更新する
- [x] 1.5 `docs/contracts/game-record.schema.json`の`winner`をnullable化し、`rulesVersion`を`2`にする
- [x] 1.6 `docs/contracts/source-differences.md`へ、元実装との盤面互換性を意図的に捨てたことを記録する

## 2. ルールエンジン

- [x] 2.1 `勝者計算`を3分岐にし、`2*色合計 == 駒数*255`でNULLを返す
- [x] 2.2 `着手適用`の色変換を`INT((2S+3)/6)`へ変える
- [x] 2.3 `状態有効判定`が`finished`かつ`winner`=NULLを有効と認めるようにする
- [x] 2.4 自己テストへ、余り0/1/2とS=0・S=765の境界、連続パスと盤面満杯の引き分けを追加する

## 3. 棋譜と再生

- [x] 3.1 `rules/replay/harness.nako3`の`対局終了`が`引分`を受けて勝者をNULLにする
- [x] 3.2 終了行の生成・走査・照合を`引分`まで含めて往復させ、進行中（終了行なし）と引き分けを取り違えないことを確認する
- [x] 3.3 `RulesVersion`を`2`にし、版`1`の棋譜が`record_unsupported_rules_version`で拒否されることを確認する
- [x] 3.4 `rules/replay/sample-game.nako3`を再生成し、注釈と再生結果が一致することを確認する

## 4. Go側の勝者表現

- [x] 4.1 `replay.Record`・`Script`・`Summary`の勝者を`*protocol.Player`にする
- [x] 4.2 `protocol`の検証が`finished`かつ`winner`=nullを受理し、`playing`では引き続き拒否することを確認する
- [x] 4.3 引き分け盤面の決定的テストをGo側にも置き、棋譜の書き出しと再生の往復で`record_mismatch`にならないことを確認する

## 5. 盤面UI

- [x] 5.1 `web/main.nako3`の勝敗ポップアップが`winner`=nullを引き分けとして表示する
- [x] 5.2 `winner`が`dark`・`light`・nullのいずれでもないときは、従来どおり結果を表示できないことを通知する

## 6. 検証

- [x] 6.1 `go vet`と`go test -race`が成功することを確認する
- [x] 6.2 `make check`を実行し、Issue #40の受け入れ条件を一つずつ確認する
