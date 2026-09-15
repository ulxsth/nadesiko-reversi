## Context

`server/main.go`は`make dev`で使う開発サーバーとして書かれている。待ち受けは`127.0.0.1:4173`固定、WebSocketは`localWebSocketOrigin`でループバック同一originだけを許可し、読み取りdeadlineは90秒、接続数・room数・command頻度の上限はない。gonakoはsubprocessとして起動し、`.tools/bin/gonako`を既定パスにしている。ブラウザランタイム`web/vendor/wnako3.js`と`.tools`はどちらもgit管理外で、`scripts/bootstrap-wsl.sh`が版を固定して取得する。

Issue #48の専有パスはコンテナ関連の新規ファイル、`server/main.go`と`server/*_test.go`、`web/index.html`と`web/main.nako3`、`docs/deployment-cloud-run.md`、そして自分のOpenSpec changeだけである。`Makefile`、`go.mod`、`scripts/bootstrap-wsl.sh`、`scripts/check-wsl.sh`、`server/internal/**`、`rules/**`は他Issueまたはコーディネータの領域なので変更しない。

## Goals / Non-Goals

**Goals:**

- 版を固定した単一イメージで、外部取得なしに公開デモを動かす。
- 公開originを1件に絞ったWebSocket受け入れにする。転送ヘッダーを信用しない。
- 無操作で切れる接続をなくし、切る場合は理由を伝えて再接続へ導く。
- 無認証の公開でも単一インスタンスが過負荷にならない上限を持つ。
- 対局が揮発することとペアリングが先着順であることを、画面と文書の両方で明示する。

**Non-Goals:**

- match/replay/runtime packageの再設計。上限と生存管理はtransport側（`server/main.go`）に閉じる。
- 永続化、招待room、複数インスタンス間の一貫性。
- CI/CDからの自動デプロイ。

## Decisions

### 構成は環境変数で受け取り、既定値はローカル開発のまま

`PORT`があれば`0.0.0.0:$PORT`、なければ`-addr`の既定値`127.0.0.1:4173`を使う。`-addr`を明示した場合はそちらを優先する。これで`make dev`の挙動は変わらず、Cloud Runのコンテナ契約も満たす。

上限と公開originも環境変数で受け取る（`PUBLIC_ORIGIN`、`MAX_CONNECTIONS`、`MAX_ROOMS`、`COMMAND_RATE_PER_SEC`、`COMMAND_BURST`、`IDLE_TIMEOUT_SECONDS`、`MAX_CONNECTION_SECONDS`）。値の解釈は純粋関数へ切り出し、`server/main_test.go`から表駆動で検証する。不正値は起動時に失敗させ、公開後に気づく事態を避ける。

### originはヘッダー1つだけで判定する

`PUBLIC_ORIGIN`は`https://<host>[:port]`の形だけを受け付け、path・query・fragment・userinfoを持つ値やhttpを起動時に拒否する。接続時は`Origin`ヘッダーをscheme小文字・host小文字へ正規化し、設定値と文字列一致した場合だけ許可する。`Host`、`X-Forwarded-Host`、`X-Forwarded-Proto`は見ない。Cloud Runの前段proxyはこれらを書き換えられるので、判定に使うと公開originの意味がなくなる。

ローカル開発の経路は既存の`localWebSocketOrigin`をそのまま残し、`PUBLIC_ORIGIN`との一致判定をORで足す。ループバック判定は`RemoteAddr`を見るため、Cloud Run経由の接続がこちらで通ることはない。

### 生存管理はping/pong、切断はcloseコードと理由つき

接続ごとに30秒間隔でping frameを送り、読み取りdeadlineは75秒（ping間隔の2倍＋余裕）にする。ブラウザは自動でpongを返すので、無操作でも接続は維持される。

そのうえで2つの上限を置く。アイドル上限（既定600秒）は「commandの受信もeventの配信もない時間」で計り、相手の長考中に切れないようにする。接続時間上限（既定3300秒）はCloud Runのrequest timeout上限60分より手前で自分から切るためのもので、`1006`の理由なし切断ではなく理由つきで閉じられる。

closeコードは、混雑と上限超過に`1013`、アイドル上限に`4001`、接続時間上限に`4002`、サーバー終了に`1001`を使う。`4001`・`4002`はapplication用の私的範囲で、ブラウザの`close`イベントからコードと理由の両方を読める。

### 上限はtransport側に置く

`match.Manager`は#5の領域なので触らない。接続数は`server/main.go`のカウンタ、room数は`Manager.RoomCount()`の読み取り、command頻度は接続ごとのtoken bucket（既定で毎秒2補充・最大8）で実装する。

接続上限とroom上限の超過は、HTTP 503ではなくupgrade後の即時closeで伝える。ブラウザのWebSocket APIはupgrade前のHTTP statusを読めず、`1006`になって理由を表示できないためである。command頻度の超過は既存の拒否response（`invalid_request`）で返し、確定stateは変えない。

既定値は同時接続20・room10とする。Managerは待機中のroomを優先して埋めるので、20接続からrooms 10を超えることは通常起きない。room上限は保険として残す。

### 終了と再起動を明示的に扱う

SIGTERM/SIGINTで`matches.Close`を呼んで全席へ終了理由を配信し、そのあと`http.Server.Shutdown`する。これでCloud Runのインスタンス停止時に、クライアントは`1006`ではなく理由を受け取れる。

再起動後に同じ参加者IDで再接続すると、Managerは新しいroomの待機席を割り当てる。クライアントは`waiting`を受け取った時点で手元の盤面を破棄し、「前の対局は終了しました」を表示する。これが「復帰不能なら失われた対局を継続中と誤表示しない」の実装で、サーバー側の追加stateを必要としない。

### `/healthz`は起動時のルール実行確認を含める

`runtime.Gonako.Ready()`は実行ファイルとルールsourceのstatだけを見る。コンテナでは同梱物が存在するのは当然なので、起動時に`newGame`を1回評価して実際に動くことを確かめ、その結果を合わせて`gonakoReady`とする。未readyのときはHTTP 503を返し、健全性確認から見て失敗と分かるようにする。評価に失敗しても起動自体は続け、`/healthz`で理由を観測できるようにする。

### イメージはbuild stageで版を固定し、runtime stageは同梱物だけを持つ

build stageは`golang:1.26.0-bookworm`でサーバーをbuildし、gonako 3.8.4のlinux-amd64 zipとブラウザランタイム3.8.1をsha256付きで取得する。runtime stageは`debian:bookworm-slim`。gonakoはsubprocessとして起動し、棋譜再生は一時ファイルを書くので、distrolessやscratchではなく最小のDebianを使う。

`scripts/bootstrap-wsl.sh`と同じ版を使うが、scriptは共有ファイルなので参照せず、Dockerfileへ独立に書く。版がずれた場合に気づけるよう、両者の版を`docs/deployment-cloud-run.md`へ並べて記す。

### デプロイscriptは冪等な2段階にする

`scripts/deploy-cloud-run.sh`は、Artifact Registryのリポジトリ作成（存在すれば何もしない）、Cloud Buildでのイメージbuild、`gcloud run deploy`、そして発行された`run.app` URLを`PUBLIC_ORIGIN`へ書き戻す2回目の更新、の順に実行する。初回は公開URLが未確定なので、この2段階が必要になる。

GCPプロジェクトIDと課金は利用者が用意する前提で、必須の環境変数が無ければ即座に止める。`--dry-run`で実行せずにコマンド列だけを出せるようにし、gcloudを持たない環境でも手順を確認できるようにする。

## Risks / Trade-offs

- [最大インスタンス1でも、デプロイ中の新旧revision併存や一時的な複数インスタンスは起こりうる] → メモリ内マッチングの完全な一貫性は保証しないと文書と仕様に明記する。トラフィック分割とtag付きrevisionは使わない。
- [アイドル上限で人を切ってしまう] → 相手の手番の間もeventで時刻を更新し、実質「両者が何もしない時間」で計る。既定600秒は文書に書き、環境変数で伸ばせるようにする。
- [接続時間上限55分より先にCloud Run側の60分で切られる可能性] → 上限値を環境変数で調整できるようにし、Cloud Run側のtimeoutを60分に設定する手順を文書へ書く。どちらで切れてもクライアントは再接続できる。
- [上限値の既定が実測ではない] → CPU 1・メモリ512MiB・concurrency 20を起点とし、gonakoの実測で調整すると文書に記す。gonakoはcommandごとにprocessを起動するため、concurrencyを上げるとCPUが先に飽和する。
- [イメージbuild時に外部から3つの資材を取得する] → sha256で固定する。実行時の取得は無い。
- [`/healthz`が未readyで503を返すのは既存の期待と違う] → bodyのfieldは維持し、statusで未readyを通知する形へspecを更新する。
