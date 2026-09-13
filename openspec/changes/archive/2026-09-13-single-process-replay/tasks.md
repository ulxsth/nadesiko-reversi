## 1. ルール本体の分離

- [x] 1.1 `rules/game/rules.nako3`へルール関数と定数を移し、トップレベルで何も実行しないことを確認する
- [x] 1.2 `rules/game/main.nako3`を取り込み・自己テスト・CLIだけにし、標準入力JSONと`--test`の出力が変わらないことを確認する
- [x] 1.3 `rules/game/README.md`をファイル分担に合わせて更新する

## 2. 1プロセス再生

- [x] 2.1 `rules/replay/harness.nako3`がルール本体を取り込み、全手の適用と各手の盤面をJSON 1件で返すようにする
- [x] 2.2 `LoadHarness`が取り込み行を絶対パスへ書き換え、一時領域からでも解決できることを確認する
- [x] 2.3 `Replayer`から`runtime.Runner`を外し、decoderの結果だけで再生結果を組み立てる
- [x] 2.4 60手の棋譜でgonako起動回数が厳密に1回であることをGoテストで確認する
- [x] 2.5 途中手数の取り出しでも起動回数が増えないことを確認する

## 3. ルール版と安全性

- [x] 3.1 `ルール版宣言`行を文法へ追加し、`Scan`が行番号を持てるようにする
- [x] 3.2 未対応の版と版宣言の欠落を、gonako起動前に行番号つきで拒否する
- [x] 3.3 `record_unsupported_rules_version`をerror codeへ加え、契約の表と件数を一致させる
- [x] 3.4 許可語彙の外を実行前に拒否することをテストで固定する
- [x] 3.5 一時ファイルが成功時も失敗時も残らないことをテストで確認する

## 4. 契約文書

- [x] 4.1 `docs/contracts/game-record.md`を新設し、文法、許可語彙、実行モデル、版互換性、error codeを記述する
- [x] 4.2 `docs/contracts/game-record.schema.json`へ`rulesVersion`を必須項目として追加する
- [x] 4.3 文書の例と`rules/replay/sample-game.nako3`とテストの例が一致することを確認する
- [x] 4.4 `rules/replay/README.md`を1プロセス再生と許可語彙に合わせて更新する

## 5. 検証

- [x] 5.1 代表的な60手棋譜の処理時間をbenchmarkで計測し、PR本文へ実測値と計測方法を記録する
- [x] 5.2 #6で追加した正常系・改ざん系・service testが通ることを確認する
- [x] 5.3 `openspec validate --all --strict`を実行し、全artifactが有効であることを確認する
- [x] 5.4 Issue #30の受け入れ条件を一つずつ照合し、`make check`が成功することを確認する
