## MODIFIED Requirements

### Requirement: 二人対戦のWebSocket境界
サーバーは許可されたoriginのブラウザから参加者IDを指定してWebSocketへ接続でき、先着二人を一つのroomへ割り当てなければならない（MUST）。許可されるのはローカルのループバック同一originと、設定された公開HTTPS origin 1件との完全一致だけである（MUST）。参加・切断・再接続の通知にはroom ID、受信者の席、利用可能な場合は現在state全体を含めなければならない（MUST）。受理したcommandの確定stateは双方へ配信し、拒否responseは送信者だけへ返さなければならない（MUST）。接続はserver発のping/pongで維持し、切断する場合は識別できるcloseコードと表示可能な日本語の理由を添えなければならない（MUST）。

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

#### Scenario: 許可されないorigin
- **WHEN** ループバック同一originでも設定された公開originでもないOriginから接続する
- **THEN** 接続を拒否し、roomを作成しない

#### Scenario: 操作のない接続
- **WHEN** 相手待ちなどでcommandの送受信がない時間が続く
- **THEN** ping/pongで接続を維持し、無操作だけを理由に切断しない

### Requirement: 開発サーバーのready確認
同一HTTPサーバーは静的画面、ローカルAPI、WebSocket、棋譜取得を提供し、`GET /healthz`でGo側とgonako側のready状態を区別して返さなければならない（MUST）。gonakoのready判定には起動時のルール実行確認を含めなければならない（MUST）。待ち受けアドレスは環境変数`PORT`が与えられたとき全インターフェースの当該port、与えられないとき`127.0.0.1:4173`でなければならない（MUST）。

#### Scenario: 起動後の確認
- **WHEN** Goサーバーが起動し、gonakoの実行ファイルを使用できる
- **THEN** `/healthz`にGoとgonakoのready状態を返し、同じoriginから画面と対戦へアクセスできる

#### Scenario: gonakoを使用できない
- **WHEN** Go側は応答できるがgonakoを使用できない
- **THEN** `/healthz`はGoの応答可能状態とgonakoの非ready状態を区別して返し、成功以外のHTTP statusで未readyを通知する

#### Scenario: PORTを与えた起動
- **WHEN** `PORT`を与えてサーバーを起動する
- **THEN** `0.0.0.0`の当該portで待ち受け、`PORT`がなければ`127.0.0.1:4173`で待ち受ける
