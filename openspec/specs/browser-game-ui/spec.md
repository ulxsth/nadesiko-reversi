# browser-game-ui Specification

## Purpose

ブラウザ盤面がサーバーの確定stateをプレイヤーへ明確に提示し、対局中から終局までの状態変化を視覚表示と支援技術の双方で理解できるようにする。

## Requirements

### Requirement: 終局結果ポップアップ
ブラウザUIは確定stateの`phase`が`finished`になったとき、勝者を示すポップアップを画面中央へ表示しなければならない（MUST）。`winner=dark`は黒、`winner=light`は白として表示し、ポップアップ内へ内部値の`dark`と`light`を露出してはならない（MUST NOT）。

#### Scenario: 黒の勝利
- **WHEN** `phase=finished`かつ`winner=dark`の確定stateを表示する
- **THEN** 画面中央へ「どちらかというと　黒　の勝利！」と表示する

#### Scenario: 白の勝利
- **WHEN** `phase=finished`かつ`winner=light`の確定stateを表示する
- **THEN** 画面中央へ「どちらかというと　白　の勝利！」と表示する

#### Scenario: 終局stateの再描画
- **WHEN** 同じ`finished` stateを複数回描画する
- **THEN** 既存のポップアップ1個の内容と表示状態を更新し、要素を重複生成しない

#### Scenario: 不正な勝者
- **WHEN** `phase=finished`だが`winner`が`dark`と`light`のどちらでもないstateを受け取る
- **THEN** 誤った勝者のポップアップを表示せず、結果を表示できないことを状態表示で通知する

### Requirement: ポップアップのライフサイクル
ブラウザUIは対局中に終局ポップアップを表示してはならず（MUST NOT）、新しい対局の開始操作時に以前の結果を非表示にしなければならない（MUST）。

#### Scenario: 新しい対局を開始する
- **WHEN** 終局ポップアップの表示中に「新しい対局」を実行する
- **THEN** 新規対局requestの完了を待たずに以前のポップアップを非表示にする

#### Scenario: 進行中stateへ戻る
- **WHEN** `phase=playing`の確定stateを表示する
- **THEN** 終局ポップアップを非表示にする

### Requirement: 終局通知のアクセシビリティ
終局結果はブラウザ標準の同期ダイアログではなく文書内の要素として表現し、支援技術が結果文全体を一度の状態変化として通知できなければならない（MUST）。

#### Scenario: 支援技術による終局検知
- **WHEN** 非表示だった終局結果が表示される
- **THEN** 結果を表す要素がlive regionとして勝敗文全体を通知する

#### Scenario: JavaScriptダイアログに依存しない
- **WHEN** 終局結果を表示する
- **THEN** `alert()`を呼び出さず、既存ページ内の要素を表示する

### Requirement: 次の駒色と手番を中心とした右側表示
ブラウザUIは確定stateの`nextColor`を盤上の駒と同じ円形・黒い輪郭・グレースケール塗りでプレビューし、隣に0〜255の数値と現在の手番を表示しなければならない（MUST）。右側UIに手数・合法手数・自分の項目を表示してはならない（MUST NOT）。

#### Scenario: 進行中stateを表示する
- **WHEN** `phase=playing`の確定stateを受け取る
- **THEN** 次の駒色を円形プレビューと数値で示し、現在の手番を表示する

#### Scenario: 色値の境界
- **WHEN** `nextColor`が0または255の確定stateを受け取る
- **THEN** どちらの色も黒い輪郭で識別できる円形プレビューと、対応する0または255の数値を表示する

#### Scenario: 終局stateを表示する
- **WHEN** `phase=finished`の確定stateを受け取る
- **THEN** 勝敗ポップアップを妨げず、右側には次の駒色と手番のみを維持する

### Requirement: requestプレビューに依存しない操作
ブラウザUIは送信JSONのプレビューを表示してはならず（MUST NOT）、プレビューの有無にかかわらず新しい対局・着手・パスの要求を既存のローカルAPIへ送信し、状態とエラーを表示しなければならない（MUST）。

#### Scenario: 新しい対局と着手
- **WHEN** プレイヤーが新しい対局を開始し、合法手へ着手する
- **THEN** request欄を表示せずに要求を送信し、確定stateの盤面・手番・次の駒色を更新する

#### Scenario: パス
- **WHEN** 合法手がない手番でプレイヤーがパスを選ぶ
- **THEN** request欄を表示せずにパス要求を送信し、確定stateに更新する

#### Scenario: 不正なseedまたは通信失敗
- **WHEN** プレイヤーが範囲外のseedを指定するか、ローカルAPIへの通信に失敗する
- **THEN** request欄に依存せず既存の状態欄へエラーを表示し、seed・新しい対局・パスの操作を残す
