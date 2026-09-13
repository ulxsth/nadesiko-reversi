## Context

`server/main.go`は現在、`localgame.Service`を`POST /api/local/runtime`へ接続し、静的ファイルと`/healthz`を配る。`web/main.nako3`は次の手番を自分として扱う1ブラウザの交代制で、`web/index.html`がなでしこを起動する。`match.Manager`は`Join`・`Submit`・`Leave`と席別の`Event` channelを公開し、`replay`は棋譜行の生成、1プロセス再生、メモリ保管庫を公開するが、これらはHTTP入口に未接続である。現行`go.mod`にWebSocket依存はない。要求はproposalと二つの差分specを参照する。

## Goals / Non-Goals

**Goals:**

- 既存のローカル対局経路を維持しつつ、同じ`make dev`プロセスへ二人対戦と完了棋譜の読み取り経路を加える。
- 参加者の席と確定stateを唯一の正本とし、受理手だけを再生可能な棋譜へ変換する。
- #7 の専有パス内で実装と手動受け入れ手順を完結させる。

**Non-Goals:**

- インターネット公開に必要な認証・永続化・水平分散、任意棋譜のアップロード。
- `match`、`replay`、`protocol`、ルール本体、`go.mod`、`Makefile`、`web/styles.css`、`README.md`の変更。

## Decisions

### 1. Go入口で既存serviceを組み立てる

`server/main.go`で一つの`runtime.Runner`を`localgame.Service`と`match.Manager`へ渡す。`replay.LoadHarness`、`NewGonakoScriptRunner`、`NewDecoder`、`NewReplayer`、`NewMemoryStore`、`NewService`を同じプロセスで組み立て、`GET /api/replays/{gameId}`から完了対局の棋譜ソースと全frameを返す。`/healthz`は既存の`status`と`gonakoReady`を残し、`goReady`を追加する。別プロセスの中継や常駐gonakoは使わない。代案の新しい内部handler packageはIssueの専有パス外なので採らない。

### 2. ローカル限定の小さなWebSocket境界

`GET /api/match?playerId=<id>`を同一origin・loopbackのブラウザ専用とし、参加者IDは認証情報ではなく`sessionStorage`に保持するタブ単位の不透明IDとする。`server/main.go`内で標準ライブラリによるupgradeとtext frameの読み書きを扱い、masked client frame、長さ上限、close/ping/pong、単一writer、origin確認を実装する。受信内容は`protocol.Command`として解釈し、`match.Manager.Submit`へ渡す。成立・待機・切断・stateの`match.Event`はそのままJSONへ、拒否は`protocol.Response`として送信者へ返す。代案の外部WebSocket libraryは`go.mod`の変更が必要でIssue範囲外、HTTP pollingはWebSocket受け入れ条件を満たさない。標準ライブラリ実装で安全性を満たせない場合は`go.mod`の所有権変更をブロッカーとして記録し、黙って範囲を広げない。

### 3. 終局までの受理手をroom単位で記録する

`match.WithSeedFunc`でroom生成時のseedを記録し、transport側に開始時刻と受理済みcommand・eventをroomごとに保持する。room単位の送信mutexで`Submit`から記録までを直列化し、`expectedTurn`順に受理手だけを並べる。終局時に`replay`の公開行生成関数から版宣言・開始・手・終了行を構成し、`Replayer.Replay`で検証した最終stateが`match`の確定stateと一致したときだけ`Service.SaveResult`で保存する。完成通知を送る側は保存の完了を待ち、UIが直後に取得しても未保存とならないようにする。拒否手は追加せず、両者が終局前に切断してroomが破棄されたら未完了の記録も捨てる。代案の`match.Room`内部へのログ追加は#5の専有範囲を再編集するため採らない。

### 4. 画面の状態機械はなでしこへ置く

`web/index.html`の短いJavaScript bridgeだけがブラウザ標準`WebSocket`と`sessionStorage`を扱い、接続・受信・切断をDOMイベントとして`web/main.nako3`へ渡す。対局モード、席、盤面描画、操作可否、日本語エラー、終局平均色、棋譜ソース表示、frame 0からの再生はなでしこ側に置く。ローカルモードは既存の`/api/local/runtime`とseed操作を維持し、二人対戦を選んだときだけWebSocketへ切り替える。既存のパネル/ボタンのクラスを再利用して`web/styles.css`を触らない。代案の全UIをJavaScriptへ移す方法はなでしこ中心という構成から外れる。

### 5. 自動検証と人間の確認を分ける

`scripts/demo-*.sh`には起動・ready・HTTP/WSの軽い契約スモークだけを置き、全対局を自動E2E化しない。`docs/manual-debug.md`に二画面の成立、交互の着手、不正手、再接続、終局、同一棋譜の先頭からの再生、/healthzを具体的に記載し、実施者・commit・環境・結果をPRへ記録してもらう。`README.md`に既にある`make bootstrap → make check → make dev`は初見PCでそのまま試し、変更が必要なら人間へ依頼する。

## Risks / Trade-offs

- [標準ライブラリだけのWebSocket実装にprotocol不備が入りやすい] → local-onlyとorigin/size制限を維持し、マスク・control frame・切断をスモークで確認する。十分に扱えなければ依存追加のブロッカーをIssueへ記録する。
- [matchの配信が終局保存より先に届く] → room単位の完了バリアで終局state/再生導線の公開を保存後まで待たせ、保存失敗は日本語で示す。
- [受理応答が並行して記録順を逆転する] → room単位で送信と記録を直列化し、turn番号の連続性と再生最終stateを照合する。
- [二人とも切断するとroomが消える] → #5 の既定どおり未完了対局を破棄し、完了した棋譜だけをメモリ保管庫へ残す。再起動後の保存は対象外。
- [初見環境のREADMEが短い] → 既存コマンドを手動で検証し、説明不足はREADMEを勝手に編集せず人間向けの残課題にする。

## Migration Plan

既存`/api/local/runtime`を変更せず、追加routeとUI操作を同時に導入する。WSLの`make check`と軽いスモークを通した後、手動デバッグの結果をfeature PRへ記録して`develop`へマージする。問題時はこのfeature PRをrevertすれば従来のローカル対局へ戻る。OpenSpec本体への同期・archiveはマージ後のコーディネータが直列で行う。
