# Cloud Runで公開デモを配信する

画面、Goサーバー、なでしこ実行系を1つのコンテナに入れ、Cloud Runの標準`run.app` URLで誰でもローカル対局と二人対戦を試せるようにする手順です。低トラフィックの公開デモが対象で、独自ドメイン、別CDN、外部DB、棋譜の永続保存は含みません。

## 同梱する版

| 対象 | 版 | 取得元 |
| --- | --- | --- |
| Go | 1.26.0 | `golang:1.26.0-bookworm` |
| gonako | 3.8.4 | GitHub releases（linux-amd64、sha256固定） |
| なでしこ3ブラウザランタイム | 3.8.1 | jsDelivr（sha256固定） |

`scripts/bootstrap-wsl.sh`が固定する版と同じです。片方を上げるときはもう片方も合わせ、この表を更新してください。取得はイメージのbuild時だけで、実行時に外部から資材を取りに行きません。`web/`と`rules/`はイメージへ同梱します。

## 前提

- 課金を有効にしたGCPプロジェクト。プロジェクトIDと課金設定は利用者が用意します。
- `gcloud`（Google Cloud CLI）でログイン済みであること。
- 必要なrole: `roles/run.admin`、`roles/artifactregistry.admin`、`roles/cloudbuild.builds.editor`、`roles/iam.serviceAccountUser`。

## ローカルでコンテナを確認する

```bash
docker build --platform linux/amd64 -t nadesiko-reversi-demo:local .
docker run --rm -p 8080:8080 -e PUBLIC_ORIGIN=http://127.0.0.1:8080 nadesiko-reversi-demo:local
```

別の端末から次を確認します。

```bash
curl -fsS http://127.0.0.1:8080/healthz    # goReady と gonakoReady が true
open http://127.0.0.1:8080/                # 画面、ローカル対局
```

二人対戦は、通常窓とプライベート窓など参加者IDが分かれる2画面で開きます。

コンテナの中から見ると、ホストからの接続元アドレスはループバックではなくDockerのbridgeになります。そのため`PUBLIC_ORIGIN`を渡さないとWebSocketが許可されません。ローカル確認のために、ループバックhostに限り`http://`の`PUBLIC_ORIGIN`を受け付けます。8080が空いていないときは`-p 18080:8080 -e PUBLIC_ORIGIN=http://127.0.0.1:18080`のように、公開する側のportへ合わせてください。

Apple Siliconなどarm64のホストでは、`--platform linux/amd64`のエミュレーションで動くため本番より遅くなります。応答速度の判断には使えません。

## デプロイする

```bash
PROJECT_ID=<GCPプロジェクトID> scripts/deploy-cloud-run.sh
```

scriptは次を順に行います。`--dry-run`を付けると、実行せずにコマンド列だけを表示します。

1. Cloud Run / Artifact Registry / Cloud BuildのAPIを有効化する
2. Artifact Registryのリポジトリを用意する（既にあれば何もしない）
3. Cloud Buildでイメージをbuildしてpushする
4. `gcloud run deploy`で公開する
5. 発行された`run.app` URLを`PUBLIC_ORIGIN`環境変数へ書き戻す

公開URLは初回デプロイまで決まらないため、5が必要です。ここを飛ばすとWebSocketのoriginが許可されず、二人対戦だけが動きません。

`gcloud builds submit`がアップロードする範囲は`.gcloudignore`、無ければ`.gitignore`で決まります。`.tools/`と`web/vendor/`は`.gitignore`にあるので、ホスト側のバイナリは送られません。

## Cloud Runの設定

| 項目 | 値 | 理由 |
| --- | --- | --- |
| region | `asia-northeast1`（東京） | 想定利用者に近い |
| URL | 標準の`run.app` | 独自ドメインは初版の対象外 |
| 認証 | 未認証アクセスを許可 | 誰でも試せるデモのため |
| 最小インスタンス | 0 | 使われていない間の費用を抑える |
| 最大インスタンス | 1 | メモリ内マッチングの分散を減らし、費用の上振れも抑える |
| concurrency | 20 | 起点の値。gonakoの実測で調整する |
| CPU / メモリ | 1 / 512Mi | 起点の値。commandごとにgonako processを起動するためCPUが先に飽和する |
| request timeout | 3600秒（60分） | WebSocketの接続期限。Cloud Runの上限 |
| HTTP/2 end-to-end | 無効 | WebSocketはHTTP/1.1のupgradeで扱う |
| トラフィック分割・tag付きrevision | 使わない | 常駐revisionが増えると費用と一貫性の両方で不利 |

## 環境変数

| 変数 | 既定 | 意味 |
| --- | --- | --- |
| `PORT` | なし | 与えると`0.0.0.0:$PORT`で待ち受ける。Cloud Runが渡す。未設定なら`127.0.0.1:4173` |
| `PUBLIC_ORIGIN` | 空 | 許可する公開origin。`https://host[:port]`だけを受け付ける（`http://`はループバックhostのローカル確認用のみ）。空ならループバック同一originのみ |
| `GONAKO_BIN` | `/app/bin/gonako` | gonakoの実行ファイル |
| `MAX_CONNECTIONS` | 20 | 同時WebSocket接続の上限 |
| `MAX_ROOMS` | 10 | 同時roomの上限 |
| `COMMAND_RATE_PER_SEC` | 2 | 1接続あたりのcommand補充数（毎秒） |
| `COMMAND_BURST` | 8 | 1接続あたりのcommandの貯め置き上限 |
| `IDLE_TIMEOUT_SECONDS` | 600 | 双方が何もしない時間の上限 |
| `MAX_CONNECTION_SECONDS` | 3300 | 1接続の継続時間の上限。Cloud Runの60分より手前で自分から切る |

不正な値を渡すとサーバーは起動しません。公開後に既定値へ落ちて気づかない状態を避けるためです。

接続の生存はサーバー発のping/pongで維持します。操作がないだけでは切れません。切断する場合は終了コードと日本語の理由を送り、画面はその理由を表示して再接続を促します。上限を超えた接続は`1013`、アイドル上限は`4001`、接続時間上限は`4002`、サーバー終了は`1001`です。

## 既知の制限

- **対局と棋譜は揮発します。** プロセス内にだけ持つので、再起動やインスタンス停止（最小インスタンス0からのscale-to-zeroを含む）で失われます。永続化は別Issueです。
- **ペアリングは先着順です。** 招待やroom指定はありません。
- **マッチングの完全な一貫性は保証しません。** 最大インスタンス1でも、デプロイ中の新旧revision併存や一時的な複数インスタンスは起こり得ます。別インスタンスへ振られた二人は同じroomになりません。
- **接続は最大60分で切れます。** Cloud Runのrequest timeoutの上限です。サーバーは手前で理由つきに切り、クライアントは再接続できます。
- 上限値は実測ではなく起点の値です。gonakoの所要時間を測ってから調整してください。

## 費用の確認と予算アラート

課金対象はCloud Runだけではありません。少なくとも次を確認してください。

- Cloud Run: リクエスト時間・CPU・メモリ。WebSocketは接続している間ずっとリクエスト時間として数えます。
- Cloud Build: buildの実行時間。
- Artifact Registry: イメージの保管容量。古いタグは消してください。
- ネットワーク: 下り（外向き）通信量。

Billingの予算アラートを設定します。

```bash
gcloud billing budgets create \
  --billing-account=<請求先アカウントID> \
  --display-name="nadesiko-reversi demo" \
  --budget-amount=1000JPY \
  --threshold-rule=percent=50 \
  --threshold-rule=percent=90 \
  --threshold-rule=percent=100
```

予算アラートは**通知であって、請求の絶対上限ではありません**。最大インスタンス数も同じく上限ではなく、同時実行数の制限です。どちらも「これ以上は課金されない」保証にはなりません。Preview機能のspend capが利用可能なら、あわせて検討してください。止めたいときはサービスを削除します。

```bash
gcloud run services delete nadesiko-reversi-demo --region asia-northeast1 --project <GCPプロジェクトID>
```

## 公開URLの手動確認

恒常的なE2Eテストは追加しません。デプロイ後に人が次を確認し、結果をPRへ記録します。

| 番号 | 操作と期待値 | PASS/FAIL・補足 |
| --- | --- | --- |
| 48-1 | `curl -fsS <公開URL>/healthz`が`goReady`と`gonakoReady`をtrueで返す | |
| 48-2 | 公開URLの画面でローカル対局の着手とパスができる | |
| 48-3 | 別端末または別ブラウザの2画面で二人対戦が成立し、両画面の盤面が一致する | |
| 48-4 | 一方の画面を閉じて再接続すると、元の席と現在盤面へ戻る | |
| 48-5 | 相手を待つ画面を数分放置しても切れない。切れた場合は画面に理由が出て、再接続できる | |
| 48-6 | 終局まで進めると両画面に同じ勝敗と平均色が出て、棋譜を再生できる | |
| 48-7 | サーバー再起動後に再接続すると「前の対局は終了しました」と出て、古い盤面が進行中として残らない | |
| 48-8 | 別originの画面からWebSocketへ接続できない（開発者ツールで確認） | |

失敗したときは、Cloud Runのログと`/healthz`の応答を合わせて記録してください。

```bash
gcloud run services logs read nadesiko-reversi-demo --region asia-northeast1 --project <GCPプロジェクトID> --limit 100
```
