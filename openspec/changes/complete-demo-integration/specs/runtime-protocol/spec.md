## ADDED Requirements

### Requirement: 二人対戦のWebSocket境界
ローカルサーバーは同一originのブラウザから参加者IDを指定してWebSocketへ接続でき、先着二人を一つのroomへ割り当てなければならない（MUST）。参加・切断・再接続の通知にはroom ID、受信者の席、利用可能な場合は現在state全体を含めなければならない（MUST）。受理したcommandの確定stateは双方へ配信し、拒否responseは送信者だけへ返さなければならない（MUST）。

#### Scenario: 待機と成立
- **WHEN** 一人目が接続し、その後別の参加者が接続する
- **THEN** 一人目に待機を通知し、成立時に双方へ席と同じ初期stateを通知する

#### Scenario: commandの受理と拒否
- **WHEN** 参加者が`expectedTurn`付きのcommandを送る
- **THEN** 受理時は両席へ更新後state全体を配信し、拒否時は送信者だけへ日本語messageと変更前stateを含むresponseを返す

#### Scenario: 不正な参加者またはメッセージ
- **WHEN** 参加者IDが欠落するか、commandをJSONとして解析できない
- **THEN** 接続または要求を拒否し、roomの確定stateを変更しない

#### Scenario: 再接続の境界
- **WHEN** 一方が切断され、残る一方が接続中に同じ参加者IDが再接続する
- **THEN** 元の席に戻して現在state全体を送る。両者が切断済みなら古いroomを再利用しない

### Requirement: 完了対局の棋譜取得
ローカルサーバーは完了対局の受理済みcommandだけから、対局時のseedとルール版を持つ実行可能ななでしこ棋譜を保存し、対局IDで棋譜ソースと初期局面を含む再生盤面列を取得できなければならない（MUST）。取得した最終stateと勝者は対局の確定結果と一致しなければならない（MUST）。

#### Scenario: 完了対局を取得する
- **WHEN** 終局したroomの対局IDで棋譜と再生結果を要求する
- **THEN** 棋譜ソース、0手目から最終手までのstate列、確定した勝者を返す

#### Scenario: 拒否されたcommand
- **WHEN** 対局中に不正手が拒否された後、対局が終了する
- **THEN** 棋譜に拒否手を含めず、再生の全盤面と最終stateは元対局に一致する

#### Scenario: 未完了または不明な対局
- **WHEN** 完了していない対局IDまたは存在しない対局IDを要求する
- **THEN** 棋譜の代わりに日本語の取得エラーを返し、他の対局の記録は返さない

### Requirement: 開発サーバーのready確認
`make dev`で起動した同一HTTPサーバーは静的画面、ローカルAPI、WebSocket、棋譜取得を提供し、`GET /healthz`でGo側とgonako側のready状態を区別して返さなければならない（MUST）。

#### Scenario: 起動後の確認
- **WHEN** Goサーバーが起動し、gonakoの実行ファイルを使用できる
- **THEN** `/healthz`にGoとgonakoのready状態を返し、同じoriginから画面と対戦へアクセスできる

#### Scenario: gonakoを使用できない
- **WHEN** Go側は応答できるがgonakoを使用できない
- **THEN** `/healthz`はGoの応答可能状態とgonakoの非ready状態を区別して返す
