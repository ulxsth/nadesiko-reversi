## ゴール

ゲーム状態をJSONで受け取り、合法手判定・着手・色変換・終了判定を行う副作用のないルールエンジンを、gonakoで実行可能ななでしこコードとして実装する。

## Agent assignment contract

- change-id: `implement-game-rules`
- branch: `feature/implement-game-rules`
- 専有パス:
  - `rules/game/**`
  - `rules/testdata/**`
  - `openspec/changes/implement-game-rules/**`
- 変更禁止:
  - `rules/smoke.nako3`
  - `server/**`
  - `web/**`
  - `openspec/specs/**`
  - 共有設定ファイル
- 依存Issue: #1（`develop`へマージ済みであること）

## 受け入れ条件

- [ ] #1の契約どおり、初期盤面、合法手一覧、着手後状態をJSONで入出力できる
- [ ] 盤端・角・複数方向を含む8方向の挟み判定がある
- [ ] 挟んだ各駒へ規定のグラデーション計算を適用する
- [ ] 空きマス以外、不成立手、範囲外、壊れたJSONを明示的に拒否する
- [ ] 同じ状態とseedから常に同じ結果が得られる
- [ ] 終了条件と勝敗境界のテストデータがある
- [ ] gonako単体のテスト用なでしこプログラムが成功する
- [ ] OpenSpecの全artifactが揃い、WSLで`make check`が成功する

## マージ条件

公開関数とJSON契約を`rules/game/`内で完結させ、サーバーへの配線は#3へ残す。feature PRではarchiveしない。
