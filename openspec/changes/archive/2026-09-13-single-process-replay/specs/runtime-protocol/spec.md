## ADDED Requirements

### Requirement: 棋譜の許可語彙
棋譜は`ルール版宣言`、`対局開始`、`黒着手`、`白着手`、`黒パス`、`白パス`、`対局終了`と、`#`で始まる注釈行および空行だけで構成されなければならない（MUST）。再生側はこれ以外の行を、なでしこ処理系を起動する前に`record_syntax_error`として拒否しなければならない（MUST）。拒否は行番号とその行のソースを伴わなければならない（MUST）。

#### Scenario: 許可語彙だけの棋譜
- **WHEN** 許可語彙と注釈行だけで構成された棋譜を読む
- **THEN** 全行を受理して対局データを返す

#### Scenario: 許可語彙の外を含む棋譜
- **WHEN** 変数代入、関数定義、取り込み、表示、ファイル操作、OS実行のいずれかを含む行がある
- **THEN** なでしこ処理系を起動せずに`record_syntax_error`と当該行の行番号を返す

#### Scenario: 手番語の綴り違い
- **WHEN** `黒`または`白`以外の手番語を使った着手行がある
- **THEN** 許可語彙の外として`record_syntax_error`を返す

### Requirement: 棋譜のルール版
棋譜は`ルール版宣言`行をちょうど1回持たなければならない（MUST）。再生側は宣言された版を、なでしこ処理系を起動する前に検査しなければならない（MUST）。再生できない版は`record_unsupported_rules_version`として、宣言行の行番号とソースを添えて拒否しなければならない（MUST）。盤面、合法手、色変換、乱数、終了条件、勝敗のいずれかの振る舞いが変わった場合、ルール版を上げなければならない（MUST）。

#### Scenario: 対応する版
- **WHEN** 再生側が再生できる版を宣言した棋譜を読む
- **THEN** 通常どおり再生する

#### Scenario: 未対応の版
- **WHEN** 再生側が知らない版を宣言した棋譜を読む
- **THEN** なでしこ処理系を起動せずに`record_unsupported_rules_version`と宣言行の行番号を返す

#### Scenario: 版宣言が無い、または重複する
- **WHEN** `ルール版宣言`行が0回、または2回以上ある
- **THEN** `record_missing_header`を返し、対局データを作らない

### Requirement: 棋譜再生の実行モデル
棋譜1本の検査、読み取り、全手の適用、各手の盤面、最終盤面、勝者の算出は、なでしこ処理系の1プロセスで完結しなければならない（MUST）。棋譜1本あたりの起動回数は手数に依存してはならない（MUST NOT）。常駐プロセス、プロセスプール、daemonを導入してはならない（MUST NOT）。

#### Scenario: 60手の棋譜
- **WHEN** 60手の棋譜を最後まで再生する
- **THEN** なでしこ処理系の起動回数はちょうど1回で、初期局面を含む61個の盤面を返す

#### Scenario: 途中手数までの取り出し
- **WHEN** 指定した手数までの盤面だけを要求する
- **THEN** 起動回数は1回のままで、棋譜全体の合法性は検証される

#### Scenario: ルールの単一の正本
- **WHEN** 同じ対局をCLIのrequest評価と棋譜再生の両方で処理する
- **THEN** 各turnの盤面、`nextColor`、勝者が一致する

### Requirement: 再生時のファイル書き込み
再生側はルール本体と再生ハーネスを読み取りのみで扱い、読み取り専用の配置で動作しなければならない（MUST）。実行用に生成するファイルはOSの一時領域だけに作らなければならず（MUST）、実行の成否にかかわらず削除しなければならない（MUST）。

#### Scenario: 再生が成功する
- **WHEN** 棋譜を最後まで再生する
- **THEN** 一時領域に作業ファイルが残らない

#### Scenario: 再生が失敗する
- **WHEN** 実行が文法エラーや異常終了で失敗する
- **THEN** 一時領域に作業ファイルが残らない

## MODIFIED Requirements

### Requirement: 対局記録
game recordはversion、rules version、game ID、seed、開始・終了時刻、最終勝者、受理済みcommand列を持たなければならない（MUST）。記録のcommand列をseedから順に再適用した結果は保存された最終stateと一致しなければならない（MUST）。実行可能な棋譜表現とJSON表現は同じrules versionを示さなければならない（MUST）。

#### Scenario: 完了対局の再生
- **WHEN** 完了したgame recordを先頭から再生する
- **THEN** 各turnと最終盤面、勝者が元対局と一致する

#### Scenario: 改ざんされた記録
- **WHEN** command欠落または不正なexpectedTurnを含むrecordを再生する
- **THEN** 最初に不整合となるcommand IDとerror codeを返す

#### Scenario: 二つの表現の一致
- **WHEN** 同じ対局を実行可能な棋譜とJSON対局記録の両方で表す
- **THEN** どちらも同じrules version、seed、勝者、指し手の並びを示す
