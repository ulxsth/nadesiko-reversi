## ゴール

再現対象のゲーム挙動と、ブラウザ・Go・gonako間のデータ契約を実装前に固定する。曖昧な点は決定事項として記録し、後続Issueが同じ前提で並行作業できる状態にする。

## Agent assignment contract

- change-id: `define-game-contract`
- branch: `feature/define-game-contract`
- 専有パス:
  - `docs/contracts/**`
  - `openspec/changes/define-game-contract/**`
- 変更禁止:
  - `openspec/specs/**`
  - アプリケーションコード全般
  - 共有設定ファイル
- 依存Issue: なし

## 決める内容

- 8×8初期盤面と座標系
- 合法手および8方向の挟み成立条件
- 0〜255の駒色と `c = (a1 + a2 + b1) / 3` の丸め規則
- 次の駒色の生成責務、乱数seed、再現性
- 終了条件と平均色による勝敗境界
- game state、command、WebSocket event、対局ログのJSON Schema
- 不正手、順番違反、切断、再接続時のエラー契約

## 受け入れ条件

- [ ] `docs/contracts/`に人間向け仕様とmachine-readableなJSON Schemaがある
- [ ] 元実装と意図的に変える点が明記されている
- [ ] 正常系、境界値、不正入力の具体例がある
- [ ] 後続Issue #2〜#7が追加判断なしで実装できる
- [ ] OpenSpecのproposal/design/specs/tasksが揃い、strict validationに成功する
- [ ] WSLで`make check`が成功する

## マージ条件

このIssueを最初に`develop`へマージする。feature PRではOpenSpec changeをarchiveしない。
