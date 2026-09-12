# runtime-protocol Specification

## Purpose
ブラウザ、Goサーバー、gonako製ルールエンジンが実装言語に依存せず同じゲーム状態とエラーを交換し、棋譜を将来も再生できるJSON契約を定義する。

## Requirements

### Requirement: version付きruntime envelope
runtimeへの1回の評価はJSON object 1件を入力し、JSON object 1件を出力しなければならない（MUST）。すべてのrequest、response、game state、recordは`version`=`1`を持たなければならない（MUST）。

#### Scenario: 対応version
- **WHEN** `version`=`1`のrequestを受け取る
- **THEN** requestを評価して同じversionのresponseを返す

#### Scenario: 未対応version
- **WHEN** `version`が欠落する、または`1`以外である
- **THEN** `unsupported_version`を返し、ゲーム状態を作成または変更しない

### Requirement: runtime action
runtimeは`newGame`と`applyCommand`を受理しなければならない（MUST）。`newGame`はgame IDと32bit seedを、`applyCommand`は完全なgame stateと1件のcommandを要求しなければならない（MUST）。

#### Scenario: 新規ゲームrequest
- **WHEN** `action`=`newGame`と有効な`seed`を受け取る
- **THEN** `ok`=`true`と初期game stateを返す

#### Scenario: applyCommandのstate欠落
- **WHEN** `action`=`applyCommand`だがstateまたはcommandがない
- **THEN** `invalid_request`を返す

### Requirement: game state表現
game stateは64要素のrow-major `board`を持ち、空きマスを`null`、駒を0〜255の整数で表現しなければならない（MUST）。さらに`gameId`、`turnNumber`、`currentPlayer`、`nextColor`、`rngState`、`consecutivePasses`、`phase`、`winner`、`legalMoves`を持たなければならない（MUST）。

#### Scenario: playing状態
- **WHEN** ゲームが進行中である
- **THEN** `winner`は`null`で、`legalMoves`は現在プレイヤーの合法座標をrow-major順で重複なく持つ

#### Scenario: finished状態
- **WHEN** ゲームが終了している
- **THEN** `phase`は`finished`、`winner`は`dark`または`light`、`legalMoves`は空配列である

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

### Requirement: responseとエラー
成功responseは`ok`=`true`、更新後state、受理したcommandに対応するeventを持たなければならない（MUST）。失敗responseは`ok`=`false`、安定したerror code、日本語message、変更前stateを持たなければならない（MUST）。

#### Scenario: 成功した着手
- **WHEN** place commandが受理される
- **THEN** eventにcommand ID、配置色、座標、変更座標と変更後色、更新後turnを含める

#### Scenario: 壊れたJSON
- **WHEN** runtime入力がJSONとして解析できない
- **THEN** processは非0終了または`invalid_json` responseで失敗を観測可能にし、診断をstderrへ出す

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

### Requirement: 対局記録
game recordはversion、game ID、seed、開始・終了時刻、最終勝者、受理済みcommand列を持たなければならない（MUST）。記録のcommand列をseedから順に再適用した結果は保存された最終stateと一致しなければならない（MUST）。

#### Scenario: 完了対局の再生
- **WHEN** 完了したgame recordを先頭から再生する
- **THEN** 各turnと最終盤面、勝者が元対局と一致する

#### Scenario: 改ざんされた記録
- **WHEN** command欠落または不正なexpectedTurnを含むrecordを再生する
- **THEN** 最初に不整合となるcommand IDとerror codeを返す
