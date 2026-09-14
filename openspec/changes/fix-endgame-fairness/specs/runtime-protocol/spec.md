## MODIFIED Requirements

### Requirement: game state表現
game stateは64要素のrow-major `board`を持ち、空きマスを`null`、駒を0〜255の整数で表現しなければならない（MUST）。さらに`gameId`、`turnNumber`、`currentPlayer`、`nextColor`、`rngState`、`consecutivePasses`、`phase`、`winner`、`legalMoves`を持たなければならない（MUST）。`winner`は`playing`のとき必ず`null`、`finished`のとき`dark`、`light`、または引き分けを表す`null`でなければならない（MUST）。

#### Scenario: playing状態
- **WHEN** ゲームが進行中である
- **THEN** `winner`は`null`で、`legalMoves`は現在プレイヤーの合法座標をrow-major順で重複なく持つ

#### Scenario: finished状態
- **WHEN** ゲームが終了して勝者が決まっている
- **THEN** `phase`は`finished`、`winner`は`dark`または`light`、`legalMoves`は空配列である

#### Scenario: 引き分けのfinished状態
- **WHEN** ゲームが引き分けで終了している
- **THEN** `phase`は`finished`、`winner`は`null`、`legalMoves`は空配列である

### Requirement: 対局記録
game recordはversion、rules version、game ID、seed、開始・終了時刻、最終結果、受理済みcommand列を持たなければならない（MUST）。最終結果は`dark`、`light`、または引き分けを表す`null`でなければならない（MUST）。記録のcommand列をseedから順に再適用した結果は保存された最終stateと一致しなければならない（MUST）。実行可能な棋譜表現とJSON表現は同じrules versionを示さなければならない（MUST）。

#### Scenario: 完了対局の再生
- **WHEN** 完了したgame recordを先頭から再生する
- **THEN** 各turnと最終盤面、勝敗が元対局と一致する

#### Scenario: 引き分け対局の往復
- **WHEN** 引き分けで終わった対局を棋譜へ書き出し、再生して読み戻す
- **THEN** 最終結果は`null`のまま一致し、不一致として拒否しない

#### Scenario: 改ざんされた記録
- **WHEN** command欠落または不正なexpectedTurnを含むrecordを再生する
- **THEN** 最初に不整合となるcommand IDとerror codeを返す

#### Scenario: 二つの表現の一致
- **WHEN** 同じ対局を実行可能な棋譜とJSON対局記録の両方で表す
- **THEN** どちらも同じrules version、seed、最終結果、指し手の並びを示す

### Requirement: 棋譜の許可語彙
棋譜は`ルール版宣言`、`対局開始`、`黒着手`、`白着手`、`黒パス`、`白パス`、`対局終了`と、`#`で始まる注釈行および空行だけで構成されなければならない（MUST）。`対局終了`の引数は`黒`、`白`、または引き分けを表す`引分`でなければならない（MUST）。再生側はこれ以外の行を、なでしこ処理系を起動する前に`record_syntax_error`として拒否しなければならない（MUST）。拒否は行番号とその行のソースを伴わなければならない（MUST）。

#### Scenario: 許可語彙だけの棋譜
- **WHEN** 許可語彙と注釈行だけで構成された棋譜を読む
- **THEN** 全行を受理して対局データを返す

#### Scenario: 引き分けの終了行
- **WHEN** 終了行が`「引分」で対局終了`である
- **THEN** 勝者のいない完了記録として受理する

#### Scenario: 許可語彙の外を含む棋譜
- **WHEN** 変数代入、関数定義、取り込み、表示、ファイル操作、OS実行のいずれかを含む行がある
- **THEN** なでしこ処理系を起動せずに`record_syntax_error`と当該行の行番号を返す

#### Scenario: 手番語の綴り違い
- **WHEN** `黒`、`白`、`引分`以外の語を使った着手行または終了行がある
- **THEN** 許可語彙の外として`record_syntax_error`を返す
