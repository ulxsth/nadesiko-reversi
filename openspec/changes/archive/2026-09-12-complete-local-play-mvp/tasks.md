## 1. ローカル対局service

- [x] 1.1 runtime Runnerを使って1対局のnewGameとapplyCommandを直列評価する
- [x] 1.2 成功時だけstateを更新し、拒否時は変更前stateを返す
- [x] 1.3 JSON endpointとruntime障害のHTTP応答を実装する

## 2. ブラウザ接続

- [x] 2.1 なでしこUIの送信境界を`POST /api/local/runtime`へ接続する
- [x] 2.2 seed付き新規ゲーム、reset、place、passを確定responseへ反映する
- [x] 2.3 hot-seatとして手番を自動交代し、日本語の拒否messageを表示する

## 3. E2Eと検証

- [x] 3.1 実serverとgonakoで不正着手がstateを変えないことを確認する
- [x] 3.2 正常着手、seed再現、passを含む終了までの対局を確認する
- [x] 3.3 OpenSpec strict validationとWSLの`make check`を成功させる
