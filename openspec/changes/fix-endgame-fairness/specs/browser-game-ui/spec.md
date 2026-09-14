## MODIFIED Requirements

### Requirement: 終局結果ポップアップ
ブラウザUIは確定stateの`phase`が`finished`になったとき、結果を示すポップアップを画面中央へ表示しなければならない（MUST）。`winner=dark`は黒、`winner=light`は白として表示し、`winner`が`null`のときは引き分けとして表示しなければならない（MUST）。ポップアップ内へ内部値の`dark`と`light`を露出してはならない（MUST NOT）。

#### Scenario: 黒の勝利
- **WHEN** `phase=finished`かつ`winner=dark`の確定stateを表示する
- **THEN** 画面中央へ「どちらかというと　黒　の勝利！」と表示する

#### Scenario: 白の勝利
- **WHEN** `phase=finished`かつ`winner=light`の確定stateを表示する
- **THEN** 画面中央へ「どちらかというと　白　の勝利！」と表示する

#### Scenario: 引き分け
- **WHEN** `phase=finished`かつ`winner=null`の確定stateを表示する
- **THEN** 画面中央へ「引き分け！」と表示する

#### Scenario: 終局stateの再描画
- **WHEN** 同じ`finished` stateを複数回描画する
- **THEN** 既存のポップアップ1個の内容と表示状態を更新し、要素を重複生成しない

#### Scenario: 不正な勝者
- **WHEN** `phase=finished`だが`winner`が`dark`、`light`、`null`のいずれでもないstateを受け取る
- **THEN** 誤った結果のポップアップを表示せず、結果を表示できないことを状態表示で通知する
