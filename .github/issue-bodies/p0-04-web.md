## ゴール

ブラウザ版なでしこ3で、ゲーム状態を描画し、プレイヤーの着手意図を送出できるデモ品質の盤面UIを作る。通信未接続でもfixtureでUIを確認できるようにする。

## Agent assignment contract

- change-id: `build-board-client`
- branch: `feature/build-board-client`
- 専有パス:
  - `web/index.html`
  - `web/main.nako3`
  - `web/styles.css`
  - `web/components/**`
  - `web/fixtures/**`
  - `openspec/changes/build-board-client/**`
- 変更禁止:
  - `web/vendor/**`
  - `server/**`
  - `rules/**`
  - `openspec/specs/**`
  - 共有設定ファイル
- 依存Issue: #1（`develop`へマージ済みであること）

## 受け入れ条件

- [ ] 8×8盤面と0〜255の駒色を状態JSONから描画する
- [ ] 自分の手番、相手待ち、接続中、終了、エラーを日本語で表示する
- [ ] 次の駒色、合法手、選択中のマスが視覚的に区別できる
- [ ] クリックまたはタップを#1のcommandへ変換する
- [ ] サーバーの確定イベントまでは盤面を確定更新しない
- [ ] 640px未満の画面とキーボード操作に対応する
- [ ] fixtureだけで主要状態を手動確認できる
- [ ] OpenSpecの全artifactが揃い、WSLで`make check`が成功する

## マージ条件

通信処理は差し替え可能な境界までにし、実WebSocketの配線は#7へ残す。feature PRではarchiveしない。
