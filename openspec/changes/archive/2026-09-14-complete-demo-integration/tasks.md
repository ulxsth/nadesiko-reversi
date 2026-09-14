## 1. Goの対戦入口

- [x] 1.1 `server/main.go`で既存runtimeを`match.Manager`と`replay.Service`にも接続し、`/healthz`へGo/gonakoのreadyを返す。`go build ./server`と`curl /healthz`で確認する。
- [x] 1.2 `server/main.go`に同一origin・サイズ制限・mask検査・close/ping/pongを持つローカルWebSocket upgradeとframe入出力を加える。`scripts/demo-ws-smoke.sh`で正常接続と不正handshake/frameの拒否を確認する。
- [x] 1.3 WebSocket接続を`Manager.Join`・`Leave`と席別`Event`へつなぎ、単一writerで送る。二接続の待機→成立→同じ初期stateと、片方の切断→同じIDで復帰をスモークで確認する。
- [x] 1.4 受信した`protocol.Command`をroom単位で直列に`Manager.Submit`へ渡し、受理stateを双方、拒否responseを送信者だけへ返す。交互の二手、不正JSON・不正手・`stale_turn`で盤面が壊れないことをスモークで確認する。

## 2. 完了棋譜と読み取りAPI

- [x] 2.1 roomのseed・開始時刻・受理済みcommand/eventだけを保持し、`replay`の公開行生成関数から棋譜ソースを組み立てる。拒否手を除きturn順と版宣言・開始行が一致することをスモークで確認する。
- [x] 2.2 終局時に棋譜を1プロセスで再生して確定stateと照合し、保存後に終局を配信する。`GET /api/replays/{gameId}`でソースとframe 0から最終盤面まで取得し、未完了・不明IDでは日本語エラーになることを確認する。

## 3. なでしこ画面

- [ ] 3.1 `web/index.html`に最小のWebSocket/`sessionStorage` bridgeと対戦・ローカル切替の操作を置き、`web/main.nako3`からイベントを受けられるようにする。二画面の待機・成立と従来のローカル新規対局が動くことをブラウザで確認する。
- [ ] 3.2 `web/main.nako3`の二人対戦モードで席を固定し、全state更新、担当手番だけの着手・パス、拒否と`stale_turn`、切断・再接続を扱う。二画面で交互の二手と不正手後の同一盤面を確認する。
- [ ] 3.3 終局の勝者と全駒の平均色、棋譜ソース、初期局面から最終盤面までの再生操作をなでしこに追加する。完了対局で二画面の結果と再生最終盤面の一致、未完了時の日本語表示を確認する。

## 4. デモ検証の引き継ぎ

- [x] 4.1 `scripts/demo-*.sh`に軽いready/HTTP/WebSocketのスモークをまとめ、`bash -n scripts/demo-*.sh`と実行結果で確認する。全局面の自動E2Eは追加しない。
- [x] 4.2 `docs/manual-debug.md`へ二画面の正常系、不正手、切断復帰、終局、棋譜再生、初見PCのREADME手順と結果記録欄を追記し、Issue #7の受け入れ条件を一対一で追えることを確認する。

## 5. 完了条件

- [ ] 5.1 人間が`docs/manual-debug.md`を二画面で実施し、対象commit・OS/ブラウザ・各項目のPASS/FAILを統合PRに記録したことを確認する。READMEの修正が必要なら人間のタスクとして残す。
- [ ] 5.2 WSLで`make check`を成功させ、Issue #7の静的配信/API/WS、マッチ・交互の盤面、不正手、終局の勝者と平均色、棋譜の先頭からの再生、ready、手動確認、初見起動、OpenSpec全artifactをPR本文で一つずつ照合する。
