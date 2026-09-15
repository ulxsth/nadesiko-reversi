## Purpose

無認証の公開デモとして配信するときの、単一コンテナの実行契約、受け入れorigin、接続の生存管理、容量上限、そして対局が揮発することの公開仕様を定める。

## ADDED Requirements

### Requirement: 単一コンテナの実行契約
公開デモは画面、HTTPサーバー、ルール実行系を1つのコンテナイメージへ同梱しなければならない（MUST）。実行時に外部から資材を取得してはならない（MUST NOT）。待ち受けアドレスは環境変数`PORT`が与えられたとき全てのインターフェースの当該portとし、与えられないときはローカル開発の既定アドレスを維持しなければならない（MUST）。

#### Scenario: PORTを与えて起動する
- **WHEN** `PORT`を与えてコンテナを起動する
- **THEN** `0.0.0.0`の当該portで待ち受け、画面・ローカル対局API・WebSocket・棋譜取得を同じoriginで提供する

#### Scenario: PORTなしで起動する
- **WHEN** `PORT`を与えずにサーバーを起動する
- **THEN** ローカル開発の既定アドレス`127.0.0.1:4173`で待ち受ける

#### Scenario: 外部ネットワークを使用できない
- **WHEN** 起動後のコンテナが外部ネットワークへ到達できない
- **THEN** 画面の表示、ローカル対局、二人対戦、棋譜再生はいずれも成功する

### Requirement: 公開originの完全一致
サーバーは、設定された公開HTTPS origin 1件と完全一致するOriginを持つWebSocket接続と、ローカルのループバック同一origin接続だけを受け入れなければならない（MUST）。受け入れ判断に`Host`および転送ヘッダーを使ってはならない（MUST NOT）。公開originが設定されていないときはループバック同一originだけを受け入れなければならない（MUST）。公開originにhttpsでないschemeを設定できるのは、ループバックhostの場合だけでなければならない（MUST）。

#### Scenario: 公開originからの接続
- **WHEN** 設定された公開HTTPS originと完全に一致するOriginでWebSocketへ接続する
- **THEN** 接続を受け入れて通常どおりroomへ割り当てる

#### Scenario: 一致しないorigin
- **WHEN** scheme、host、portのいずれかが公開originと異なるOriginで接続する
- **THEN** 接続を拒否し、roomを作成しない

#### Scenario: 偽装されたヘッダー
- **WHEN** Originは一致しないが`Host`や`X-Forwarded-Host`に公開originのhostを詰めて接続する
- **THEN** ヘッダーを信用せず接続を拒否する

#### Scenario: 公開origin未設定
- **WHEN** 公開originを設定せずに起動し、ループバック以外から接続する
- **THEN** 接続を拒否し、ローカルの同一origin接続だけを受け入れる

#### Scenario: ループバックのhttp origin
- **WHEN** コンテナをローカルで確認するために、ループバックhostのhttp originを設定して起動する
- **THEN** そのoriginからの接続を受け入れる。ループバック以外のhostにはhttpを設定できない

### Requirement: 接続の生存管理と切断理由
サーバーはWebSocketへ定期的にpingを送り、pongで生存を確認しなければならない（MUST）。操作がないという理由だけで生存している接続を切ってはならない（MUST NOT）。アイドル上限、接続時間上限、サーバー終了のいずれで切る場合も、識別できるcloseコードと表示可能な日本語の理由を添えなければならない（MUST）。

#### Scenario: 操作がない接続
- **WHEN** 相手を待つなどしてcommandもeventも発生しない時間が、ping間隔を超えてアイドル上限未満の間続く
- **THEN** ping/pongで接続を維持し、切断しない

#### Scenario: アイドル上限
- **WHEN** commandの受信もeventの配信もない時間がアイドル上限を超える
- **THEN** アイドルを示すcloseコードと日本語の理由を送ってから切断する

#### Scenario: 接続時間上限
- **WHEN** 1接続の継続時間が接続時間上限に達する
- **THEN** 再接続を促す日本語の理由を送ってから切断し、同じ参加者IDでの再接続を妨げない

#### Scenario: サーバー終了
- **WHEN** サーバーが終了signalを受け取る
- **THEN** 参加中の全roomへ終了理由を配信してから接続を閉じる

### Requirement: 公開デモの容量上限
サーバーは同時WebSocket接続数、同時room数、1接続あたりのcommand頻度に上限を持たなければならない（MUST）。上限を超えたrequestは黙って捨てず、表示可能な日本語の理由を添えて拒否しなければならない（MUST）。上限超過によってすでに成立している対局の確定stateを変更してはならない（MUST NOT）。

#### Scenario: 同時接続の上限
- **WHEN** 同時接続数が上限に達した状態で新しいWebSocket接続が来る
- **THEN** 混雑を示すcloseコードと日本語の理由を返して接続を閉じ、既存の対局は影響を受けない

#### Scenario: roomの上限
- **WHEN** 同時room数が上限に達した状態で新しい参加者が接続する
- **THEN** 日本語の理由を返して接続を閉じ、新しいroomを作成しない

#### Scenario: commandの頻度超過
- **WHEN** 1つの接続が短時間に上限を超えるcommandを送る
- **THEN** 超過分を拒否responseで返し、roomの確定stateを変更しない

### Requirement: 揮発する対局と先着ペアリング
公開デモは対局と棋譜をプロセス内にだけ保持し、再起動やインスタンス停止で失われることを公開仕様として示さなければならない（MUST）。ペアリングは先着順とし、招待やroom指定を提供してはならない（MUST NOT）。一時的に複数インスタンスが存在しうるため、マッチングの完全な一貫性を保証してはならない（MUST NOT）。

#### Scenario: 再起動後の再接続
- **WHEN** 対局中にサーバーが再起動し、同じ参加者IDで再接続する
- **THEN** 失われた対局を進行中として復元せず、新しい相手を待つ状態として扱う

#### Scenario: 先着ペアリング
- **WHEN** 相手を指定せずに二人が続けて参加する
- **THEN** 先着の二人を同じroomへ入れ、招待やroom指定の手段は提供しない
