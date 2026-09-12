## 1. 人間向け契約

- [x] 1.1 `docs/contracts/game-rules.md`へ盤面、合法手、色変換、乱数、終了、勝敗を記述し、specの全Requirementとの対応を確認する
- [x] 1.2 `docs/contracts/protocol.md`へruntime envelope、command、state、event、error code、recordを記述し、正常系と失敗系の例を確認する
- [x] 1.3 `docs/contracts/source-differences.md`へ元実装との差分と判断理由を記述し、後続Issueの非目標が明確であることを確認する

## 2. Machine-readable契約

- [x] 2.1 game state、runtime request/response、game recordのJSON Schemaを作り、すべてのJSONファイルをparserで読み込めることを確認する
- [x] 2.2 seed=1のcanonical exampleとinvalid command exampleを作り、文書化したLCG値・初期盤面・error codeと一致することを確認する

## 3. 検証

- [x] 3.1 `openspec validate --all --strict`を実行し、全artifactが有効であることを確認する
- [x] 3.2 Issue #1の受け入れ条件を照合し、WSLで`make check`が成功することを確認する
