## 1. エンジン実装

- [x] 1.1 JSON入出力とversion/action validationを実装し、newGame固定vectorが期待stateと一致することを確認する
- [x] 1.2 8方向走査とrow-major合法手一覧を実装し、初期盤面12手・盤端・連続数不足を確認する
- [x] 1.3 placeの同時色変換と手番・LCG更新を実装し、seed=1の`(2,2)`着手vectorを確認する
- [x] 1.4 pass、終了、勝敗、不正commandの原子性を実装し、成功・拒否testを確認する

## 2. Fixtureと検証

- [x] 2.1 `rules/testdata/**`へ正常・境界・不正requestを追加し、gonako CLIのstdoutが有効なJSON 1件であることを確認する
- [x] 2.2 gonako製test runnerを追加し、すべてのASSERTが成功することを確認する
- [x] 2.3 `openspec validate --all --strict`とWSLの`make check`を実行し、Issue #2の受け入れ条件を満たすことを確認する
