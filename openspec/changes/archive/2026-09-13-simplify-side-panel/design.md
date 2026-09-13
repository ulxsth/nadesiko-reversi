## Context

現在の`web/index.html`は次色の四角い`#next-color-swatch`、数値、手番・手数・合法手・自分の`dl`、requestプレビューを右側に置く。`web/main.nako3`は確定state適用時に各DOMを更新し、要求送信時にもプレビューへJSONを書き込む。盤上の駒は半径32px、グレースケール塗り、2pxの黒い輪郭で描画される。目的はproposal.md、表示契約は`specs/browser-game-ui/spec.md`を参照する。

## Goals / Non-Goals

**Goals:**

- 次色プレビューを盤上の駒と同じ直径64px・黒い輪郭の円へ揃える
- DOMと更新処理を対にして整理し、削除済み要素への参照を残さない
- 通信とルール操作の経路を変えずに表示だけを簡素化する
- Issue #35の専有パス内で完結させる

**Non-Goals:**

- 盤上の駒描画、確定state、API契約の変更
- パス可否や着手コマンドを計算する内部値の削除
- #7のroute統合や#34の終局ポップアップの改修

## Decisions

### CSSの円で盤上の駒と揃える

既存の`#next-color-swatch`を維持し、CSSで直径64px・2pxの黒い輪郭・`border-radius: 50%`を指定する。塗りは従来どおり`nextColor`から生成する`rgb(n,n,n)`を使い、数値は隣に残す。canvasへ別の円を描く案は表示専用canvasと描画コードを増やすため採用しない。全体の角丸なし規則に対して、この指定は駒そのものの円形表現に限る。

### 表示用DOMと更新だけを対で削除する

`web/index.html`から手数・合法手・自分の`dt/dd`とrequestセクションを除き、`web/main.nako3`から対応する`#meta-*`と`#command-preview`更新を除く。requestプレビューのためだけの`送信要求`と整形JSONも不要なら消す。一方、`turnNumber`はcommandの`expectedTurn`、`legalMoves`は着手判定・パス可否に必要なので維持する。UIをCSSで隠す案は不要DOMと更新コストが残るため採用しない。

### 通信境界はそのまま維持する

`ルール要求送信`のpayload生成・`POST /api/local/runtime`・response処理を残す。プレビュー更新だけを外すことで、既存の新規対局・着手・パスと状態・エラー表示を保つ。送信内容を別のデバッグUIへ移す案は「requestはいらない」というIssueの意図に反するため採用しない。

### レスポンシブと専有範囲

既存の1列化ブレークポイント、パネル構造、終局ポップアップの中央配置は維持する。編集は`web/index.html`、`web/main.nako3`、`web/styles.css`、`openspec/changes/simplify-side-panel/**`に限定し、#7や他のIssueの専有パス、`openspec/specs/**`へは触れない。#34のマージ後に着手し、#7より先にマージする。

## Risks / Trade-offs

- [円が小さい画面で情報欄を圧迫する] → 次色行だけを64pxにし、幅640px未満の1列レイアウトを実ブラウザで確認する
- [白の駒が白いパネル背景へ沈む] → 盤上と同じ2pxの黒輪郭を維持し、色値0と255の境界を確認する
- [表示欄の削除とともにパス判定を壊す] → `legalMoves`計算は残し、パス可能stateと実API着手を手動確認する
- [requestプレビュー削除で通信障害が見えなくなる] → 既存の状態・エラー欄を残し、通信失敗時の表示を確認する

## Migration Plan

1. 表示DOMとCSSを整理する
2. なでしこの表示専用更新を削り、通信経路と入力判定を残す
3. WSLの`make check`とブラウザで初期表示・着手・パス・終局・狭幅を確認する

3ファイルだけの表示変更なので、問題時はfeatureコミットをrevertして従来の右側UIへ戻せる。
