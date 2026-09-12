## 1. 契約文書の更新

- [x] 1.1 `docs/contracts/protocol.md`のTransport節を書き換え、`sequence`の記述を削除して`turnNumber`による順序識別、`expectedTurn`による重複と再送の扱い、成功時の全state配信を記述する
- [x] 1.2 同文書のRuntime response節へ`commandId`の用途が相関とログに限定されることを追記し、`stale_turn`をクライアントが再同期として扱う旨を記述する
- [x] 1.3 error code表の`stale_turn`の行が、重複・再送・競合の三つを担うことと矛盾しないか確認する

## 2. Spec delta

- [x] 2.1 `specs/runtime-protocol/spec.md`のREMOVEDが`transport eventの順序`のRequirement名と完全一致することを確認する
- [x] 2.2 MODIFIEDの`command表現`が既存Requirementの全内容を含み、欠落がないことを確認する
- [x] 2.3 追加した2件のRequirementが成功系、重複・再送、境界（取りこぼしと再接続）、拒否系のScenarioを持つことを確認する

## 3. 検証

- [x] 3.1 `openspec validate --all --strict`を実行し、全artifactが有効であることを確認する
- [x] 3.2 リポジトリ全体を`sequence`で検索し、`docs/**`と`openspec/specs/**`以外に残存参照がないこと、および`openspec/specs/**`はarchive待ちであることを確認する
- [x] 3.3 Issue #22の受け入れ条件6件を一つずつ照合し、WSLで`make check`が成功することを確認する
