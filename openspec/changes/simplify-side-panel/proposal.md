## Why

現行の右側UIは次の駒色が小さな四角で判別しづらく、プレイ中に不要な手数・合法手数・自分・request JSONが主要操作と同じ領域を占めている。展示時に次の一手へ必要な情報を素早く読めるよう、駒色・手番・操作へ表示を絞る。

## What Changes

- 次の駒色を、盤上の駒と同じ円形・黒い輪郭・グレースケール塗りのプレビューへ変更する
- 色プレビュー横の0〜255の数値と、右側の手番表示は維持する
- **BREAKING** 右側UIから手数、合法手数、自分の表示を削除する
- **BREAKING** request見出しと送信JSONプレビューを削除する
- 削除したDOMへの更新処理とrequestプレビュー専用CSSを除去する一方、request payload生成と`POST /api/local/runtime`通信は維持する
- seed、新しい対局、パス、状態・エラー表示、終局ポップアップ、レスポンシブ配置は維持する

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `browser-game-ui`: 右側UIに表示するプレイ情報を次の駒色と手番へ限定し、次の駒色を盤上の駒と同じ視覚表現で提示する要件を追加する

## Impact

- GitHub Issue: #35
- feature branch: `feature/simplify-side-panel`
- 依存Issue: #34 / PR #36（`develop`へマージ済み）
- 統合順序: 同じWebファイルを変更する#7より先に`develop`へマージする
- 専有パス:
  - `web/index.html`
  - `web/main.nako3`
  - `web/styles.css`
  - `openspec/changes/simplify-side-panel/**`
- `server/**`、`rules/**`、API payload、state schema、`openspec/specs/**`は変更しない

## Non-goals

- ゲームルール、色生成、勝敗判定、通信契約の変更
- 右側パネル全体の廃止や盤面内への操作UI移設
- seed、新しい対局、パス、状態・エラー表示、終局ポップアップの再設計
- #7が扱う対戦・リプレイrouteの統合
