## ADDED Requirements

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
