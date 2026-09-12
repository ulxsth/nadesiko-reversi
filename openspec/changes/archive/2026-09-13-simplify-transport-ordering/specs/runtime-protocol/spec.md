## ADDED Requirements

### Requirement: expectedTurnによるcommand順序と冪等性
transportは受理したcommand 1件ごとに`turnNumber`をちょうど1増やさなければならない（MUST）。現在の`turnNumber`と一致しない`expectedTurn`を持つcommandは`stale_turn`として拒否し、ゲーム状態を変更してはならない（MUST NOT）。transportはcommandの重複排除のために`commandId`を記憶してはならない（MUST NOT）。

#### Scenario: 重複したcommand
- **WHEN** 同一の`expectedTurn`を持つcommandが続けて2件届く
- **THEN** 1件目を受理して`turnNumber`を1増やし、2件目を`stale_turn`で拒否して状態を変えない

#### Scenario: 応答消失後の再送
- **WHEN** クライアントが応答を受け取れずに同じcommandを再送する
- **THEN** サーバーが適用済みなら`stale_turn`と現在stateを返し、未適用なら通常どおり受理する

#### Scenario: 二人が同時に送る
- **WHEN** 二つの接続が同じ`expectedTurn`を持つcommandをほぼ同時に送る
- **THEN** 先に評価された1件だけを受理し、残りを`stale_turn`で拒否する

#### Scenario: クライアントのstale_turn処理
- **WHEN** クライアントが`stale_turn`を受け取る
- **THEN** エラーとして表示せず、応答に含まれる現在stateで表示を置き換える

### Requirement: 成功時の全state配信
transportはcommandの受理ごとに、差分ではなくgame state全体をroomの全接続へ配信しなければならない（MUST）。クライアントは受信したstateで表示を置き換えなければならない（MUST）。transportはevent欠落を検出するための連番その他の機構を設けてはならない（MUST NOT）。

#### Scenario: 配信の取りこぼし
- **WHEN** クライアントが配信を1通受け取り損ねる
- **THEN** 次の受理で配信されるstate全体によって表示が正しい状態へ追いつく

#### Scenario: 接続直後
- **WHEN** クライアントがroomへ接続する、または再接続する
- **THEN** 専用の再同期requestを必要とせず、現在のgame state全体を1通配信する

#### Scenario: 拒否されたcommand
- **WHEN** commandが拒否される
- **THEN** roomへの配信を行わず、送信者へだけ変更前stateを含むerrorを返す

## MODIFIED Requirements

### Requirement: command表現
commandは`commandId`、`player`、`expectedTurn`を持ち、`type`=`place`なら`row`と`col`を、`type`=`pass`なら座標を持たない形でなければならない（MUST）。`commandId`はeventとの相関とログ出力のためだけに使わなければならず、受理と拒否の判断に使ってはならない（MUST NOT）。

#### Scenario: place command
- **WHEN** `type`=`place`で0〜7のrowとcolを持つ
- **THEN** 手番、turn、合法性を検証して評価する

#### Scenario: 不明なcommand type
- **WHEN** `type`が`place`または`pass`以外である
- **THEN** `invalid_command`を返す

#### Scenario: commandIdが重複する
- **WHEN** 同じ`commandId`を持つcommandが2件届く
- **THEN** `commandId`の一致を理由に拒否せず、`expectedTurn`だけで受理と拒否を決める

## REMOVED Requirements

### Requirement: transport eventの順序
**Reason**: `sequence`はゲームeventだけを数える限り`turnNumber`と常に同値であり、冗長である。重複と欠落の検出という役割も、`expectedTurn`による`stale_turn`拒否と成功時の全state配信で満たせる。二つの機構が同じ不変条件を担保する状態を避ける。
**Migration**: event順序の識別には`turnNumber`を使う。重複と再送は`expectedTurnによるcommand順序と冪等性`が、欠落の吸収は`成功時の全state配信`が引き継ぐ。ローカルHTTP transportがruntime responseと同じbodyを使う点も後者が定める。
