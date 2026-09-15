## Why

統合デモは`make dev`で動くが、遊べるのはリポジトリを取得してWSLでbootstrapできる人だけになっている。画面・Goサーバー・gonakoを1つのコンテナへ入れてCloud Runへ置けば、URLを開くだけでローカル対局と二人対戦を試せる。

一方で現在のサーバーは同一originのループバック接続しか受け付けず、WebSocketは90秒無操作で切れ、同時接続数の上限もない。無認証で公開する前に、これらを公開前提の挙動へ直す必要がある。

## What Changes

- Go 1.26.0、gonako 3.8.4、ブラウザランタイム3.8.1を固定したLinux amd64コンテナを追加し、`web/`と`rules/`を同梱して実行時の外部ダウンロードをなくす。
- **BREAKING**: WebSocketの受け入れoriginを、ローカルのループバック同一originに加えて「設定された公開HTTPS origin1件との完全一致」だけに広げる。`Host`や`X-Forwarded-*`は判断に使わない。
- 待ち受けアドレスを`PORT`環境変数から決め、Cloud Runでは`0.0.0.0:$PORT`、未設定のローカル開発では従来どおり`127.0.0.1:4173`にする。
- WebSocketにserver発のping/pongを入れて無操作での切断をなくし、代わりにアイドル上限と接続時間上限を明示的な切断理由つきで設ける。
- 同時接続数・room数・command頻度に上限を設け、超過を日本語の理由つきで拒否する。
- SIGTERMで全roomへ理由を配信してから終了し、クライアントは切断理由を表示して再接続できる。復帰できなかった対局を進行中として表示しない。
- `/healthz`が起動時のgonako実行確認を含むready状態を返し、未readyを識別できるようにする。
- デプロイ手順書とデプロイscriptを追加し、費用の上限管理と「対局は揮発する」公開仕様を明記する。

## Capabilities

### New Capabilities

- `public-demo-service`: 無認証の公開デモとして配信するときの、コンテナの実行契約、受け入れorigin、接続の生存管理、容量上限、状態の揮発性。

### Modified Capabilities

- `runtime-protocol`: 「二人対戦のWebSocket境界」の受け入れoriginと生存管理、「開発サーバーのready確認」の待ち受けアドレスとready応答。
- `browser-game-ui`: 「二人対戦の確定状態と拒否の表示」に、切断理由の提示と復帰不能時の扱いを追加する。

## Impact

- GitHub Issue: #48
- Branch: `feature/deploy-cloud-run-public-demo`
- Dependencies: #7（PR #45）、#46（PR #51）。いずれも`develop`へマージ済み。
- Exclusively owned paths: `Dockerfile`、`.dockerignore`、`scripts/deploy-cloud-run.sh`、`server/main.go`、`server/*_test.go`、`web/index.html`、`web/main.nako3`、`docs/deployment-cloud-run.md`、`openspec/changes/deploy-cloud-run-public-demo/**`
- 影響範囲: 公開デモのHTTP/WebSocket境界と配信物。ルール、protocol、match、replayのpackageは変更しない。

## Non-goals

- カスタムドメイン、別CDN、ログイン、ランキングは扱わない。
- Redis/Firestoreなどの共有状態や棋譜の永続保存は行わない。対局はプロセスとともに消える。
- 常時稼働・無停止デプロイ・トラフィック分割・tag付き常駐revisionは保証も使用もしない。
- GitHub ActionsからのCI/CDデプロイは別Issueとする。
- 恒常的なE2Eテストは追加しない。公開URLの確認は人間の手動デバッグで行う。
