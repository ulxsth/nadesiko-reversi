## 1. サーバーの公開構成

- [x] 1.1 `PORT`から待ち受けアドレスを決め、未設定なら`127.0.0.1:4173`、`-addr`の明示指定を優先することをtestで確認する
- [x] 1.2 `PUBLIC_ORIGIN`を`https://host[:port]`だけ受け付ける形で解釈し（httpはローカル確認用にループバックhostのみ許す）、path・query・userinfoと公開hostのhttpを起動時に拒否することをtestで確認する
- [x] 1.3 上限値の環境変数（接続数、room数、command頻度、アイドル上限、接続時間上限）を解釈し、不正値で起動を失敗させる

## 2. WebSocketの受け入れと生存管理

- [x] 2.1 公開originとの完全一致、ループバック同一origin、偽装`Host`・`X-Forwarded-Host`の拒否をtestで確認する
- [x] 2.2 30秒間隔のping送信と75秒の読み取りdeadlineを入れ、無操作の接続が切れないようにする
- [x] 2.3 アイドル上限と接続時間上限を、closeコードと日本語の理由つきの切断として実装する
- [x] 2.4 SIGTERM/SIGINTで全roomへ終了理由を配信してからHTTPサーバーを停止する

## 3. 容量上限

- [x] 3.1 同時接続数の上限を超えた接続を、`1013`と日本語の理由で閉じる
- [x] 3.2 room数の上限を超えた参加を、`1013`と日本語の理由で閉じる
- [x] 3.3 接続ごとのtoken bucketでcommand頻度を制限し、超過を拒否responseで返して確定stateを変えないことをtestで確認する

## 4. 画面の再接続表示

- [x] 4.1 `close`のコードと理由をブラウザ側の橋へ渡し、切断理由を日本語で表示する
- [x] 4.2 `waiting`受信時に手元の盤面を破棄し、失われた対局を進行中として表示しない
- [x] 4.3 roomの`closed`通知を扱い、終局時の表示を壊さずにサーバー終了を伝える
- [x] 4.4 公開デモで対局と記録が揮発することを画面に一行で示す

## 5. コンテナ

- [x] 5.1 Go 1.26.0、gonako 3.8.4、ブラウザランタイム3.8.1をsha256付きで固定したlinux/amd64イメージをbuildする
- [x] 5.2 `web/`と`rules/`を同梱し、実行時に外部取得が無いことを確認する
- [x] 5.3 `.dockerignore`でbuild contextから`.git`、`.tools`、`web/vendor`、文書類を除く
- [x] 5.4 ローカルでコンテナを起動し、`/healthz`、ローカル対局、WebSocket二画面対戦を確認する

## 6. デプロイ手順

- [x] 6.1 `scripts/deploy-cloud-run.sh`にArtifact Registry準備、Cloud Build、`gcloud run deploy`、`PUBLIC_ORIGIN`書き戻しを実装する
- [x] 6.2 `--dry-run`でコマンド列だけを出力できるようにし、gcloud不在の環境で確認する
- [x] 6.3 `docs/deployment-cloud-run.md`に設定値、費用の確認とBilling予算アラート、既知の制限、手動デバッグ手順を書く

## 7. 検証

- [x] 7.1 `go test ./...`と`make check`が成功することを確認する
- [x] 7.2 `openspec validate --all --strict`が成功することを確認する
- [x] 7.3 Issue #48の受け入れ条件を一つずつPR本文で確認する
