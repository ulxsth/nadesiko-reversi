# 実装境界

再現対象は、8×8の盤面、0〜255のグレースケール駒、挟んだ駒の色変換、二人対戦、対局の保存・再生を持つグラデーションリバーシです。元実装のGo/WasmとAWS固有部分は移植せず、ゲームの体験をなでしこ中心の構成で再現します。

## コンポーネント

| 領域 | 責務 | 主な実装 |
| --- | --- | --- |
| `rules/game/**` | 合法手、8方向の挟み、色変換、終了・勝敗 | gonakoで実行するなでしこ |
| `server/internal/runtime/**` | Goからgonakoを安全に呼ぶ境界 | Go |
| `server/internal/protocol/**` | HTTP/WebSocketのJSON契約 | Go |
| `server/internal/match/**` | 待機、2人マッチ、手番、盤面の確定 | Go |
| `server/internal/replay/**` | 対局ログの保存・取得 | Go |
| `web/**` | 盤面描画、入力、接続状態、再生UI | ブラウザ版なでしこ3 |
| `docs/manual-debug.md` | ブラウザ・サーバー・gonako間の受け入れ確認 | 人間によるデバッグ |

## 依存方向

```text
web ──JSON/WebSocket──> server/protocol ──> server/match
                                │                  │
                                └──> server/runtime ──> rules/game
                                                   │
                                                   └──> server/replay
```

サーバーを盤面の正本とし、クライアントは確定イベントを受け取って描画します。ルールは副作用のない入力JSON→出力JSONとして切り出し、同じ対局ログを再実行すれば同じ盤面になる構成を目指します。

常時実行する自動テストは、ルール・protocol・runtime・serviceのpackage testへ限定します。実server、gonako、ブラウザをまたぐ確認は、変更の影響範囲に応じて[手動デバッグ手順](./manual-debug.md)を人が実施します。

## P0の切り方

最初に契約を固定し、その後ルール、サーバー境界、UIを独立に進めます。リアルタイム対戦とリプレイは契約済みパッケージへ閉じ、最後の統合Issueだけが既存エントリポイントを変更します。詳細は[P0バックログ](./p0-backlog.md)を参照してください。
