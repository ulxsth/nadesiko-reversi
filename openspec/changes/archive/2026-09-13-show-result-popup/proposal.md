## Why

現在は終局しても右側パネルの小さな文言だけが変わるため、プレイヤーが勝敗を見落としやすい。ハッカソン展示で終局を明確に伝えるため、勝者を画面中央のポップアップで通知する。

## What Changes

- `phase=finished` の確定stateを受け取ったとき、画面中央に勝敗ポップアップを表示する
- `winner=dark` を黒、`winner=light` を白へ変換し、「どちらかというと　黒/白　の勝利！」と表示する
- 同じ終局stateの再描画でポップアップを重複生成せず、新しい対局の開始時に非表示へ戻す
- `alert()`ではなくDOMとARIA属性で、視覚表示と支援技術への通知を両立する

## Capabilities

### New Capabilities

- `browser-game-ui`: ブラウザ盤面が確定stateを表示し、終局時に勝敗を明確かつアクセシブルに通知する振る舞い

### Modified Capabilities

なし。

## Impact

- GitHub Issue: #34
- feature branch: `feature/show-result-popup`
- 依存Issue: #10（`develop`へマージ済み）
- 専有パス:
  - `web/index.html`
  - `web/main.nako3`
  - `web/styles.css`
  - `openspec/changes/show-result-popup/**`
- サーバーAPI、ゲームルール、state schemaは変更しない
- 後続の#35は本変更のマージ後に同じWebファイルを整理する

## Non-goals

- 勝敗判定や平均色計算の変更
- ポップアップを閉じる専用ボタンやアニメーションの追加
- 対戦・リプレイrouteの統合
- 右側UIとrequestプレビューの整理（#35で扱う）
