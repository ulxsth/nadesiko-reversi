## Why

#46 の人間レビューで、画面に内部値や専門語が露出し、状態説明も重複していることが確認された。承認済みの文言整理と、平均色を数値ではなく駒と色域で伝える UI を、ルールや通信契約を変えずに実装する。

## What Changes

- 「二人対戦」「ローカル対局へ戻る」の名称と指定された勝利文言を維持し、その他の承認済み表示を初見向けに簡潔化する。
- 開発者向け seed 入力を通常画面から隠し、手動デバッグで使える経路を残す。
- 次の駒色の 0〜255 の常時数値表示をやめ、円形プレビューを主にする。
- 終局時の全駒平均色を、白から黒へ並ぶ色域上に実際の平均色で塗った円形の駒として配置する。数値と駒数は常時表示しない。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `browser-game-ui`: 画面用語、開発者向け入力の露出、次の駒色と終局平均色の表示を整理する。

## Impact

- GitHub Issue: #46。依存: #7 / PR #45 と OpenSpec archive PR #49 の `develop` マージ。ブランチ: `feature/simplify-ui-language`。
- 専有パス: `web/index.html`、`web/main.nako3`、`web/styles.css`、`docs/manual-debug.md`、`openspec/changes/simplify-ui-language/**`。
- #48 の Cloud Run 対応と #47 の画面遷移は後続・別スコープであり、同時にこれらの web ファイルを編集しない。

## Non-goals

- 駒色生成、平均値・勝者の計算、ルール、WebSocket/棋譜の通信契約、指定された勝利演出の変更。
- README の編集、サーバーの日本語メッセージの全面変更、常設の E2E テスト追加。
