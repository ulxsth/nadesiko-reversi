## Purpose

ブラウザ、Goサーバー、gonako製ルールエンジンが実装言語に依存せず同じゲーム状態とエラーを交換し、棋譜を将来も再生できるJSON契約を定義する。

## ADDED Requirements

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
commandは`commandId`、`player`、`expectedTurn`を持ち、`type`=`place`なら`row`と`col`を、`type`=`pass`なら座標を持たない形でなければならない（MUST）。

#### Scenario: place command
- **WHEN** `type`=`place`で0〜7のrowとcolを持つ
- **THEN** 手番、turn、合法性を検証して評価する

#### Scenario: 不明なcommand type
- **WHEN** `type`が`place`または`pass`以外である
- **THEN** `invalid_command`を返す

### Requirement: responseとエラー
成功responseは`ok`=`true`、更新後state、受理したcommandに対応するeventを持たなければならない（MUST）。失敗responseは`ok`=`false`、安定したerror code、日本語message、変更前stateを持たなければならない（MUST）。

#### Scenario: 成功した着手
- **WHEN** place commandが受理される
- **THEN** eventにcommand ID、配置色、座標、変更座標と変更後色、更新後turnを含める

#### Scenario: 壊れたJSON
- **WHEN** runtime入力がJSONとして解析できない
- **THEN** processは非0終了または`invalid_json` responseで失敗を観測可能にし、診断をstderrへ出す

### Requirement: transport eventの順序
WebSocket transportはruntimeの成功eventへroom単位で単調増加する`sequence`を付け、クライアントが重複と欠落を検出できるようにしなければならない（MUST）。ローカルHTTP transportも同じresponse bodyを使用しなければならない（MUST）。

#### Scenario: 二つの成功event
- **WHEN** 一つのroomで二つのcommandが順に成功する
- **THEN** 後のeventの`sequence`は前より1大きい

#### Scenario: command拒否
- **WHEN** commandが拒否される
- **THEN** 成功eventを発行せず`sequence`を進めない

### Requirement: 対局記録
game recordはversion、game ID、seed、開始・終了時刻、最終勝者、受理済みcommand列を持たなければならない（MUST）。記録のcommand列をseedから順に再適用した結果は保存された最終stateと一致しなければならない（MUST）。

#### Scenario: 完了対局の再生
- **WHEN** 完了したgame recordを先頭から再生する
- **THEN** 各turnと最終盤面、勝者が元対局と一致する

#### Scenario: 改ざんされた記録
- **WHEN** command欠落または不正なexpectedTurnを含むrecordを再生する
- **THEN** 最初に不整合となるcommand IDとerror codeを返す
