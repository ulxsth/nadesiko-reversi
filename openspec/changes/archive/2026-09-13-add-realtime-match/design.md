## Context

`openspec/specs/runtime-protocol/spec.md`の「expectedTurnによるcommand順序と冪等性」「成功時の全state配信」が外部挙動の正本である。#22で`sequence`と`commandId`による重複排除が廃止され、順序は`turnNumber`、競合と再送は`expectedTurn`へ一本化された。

Issue #5の専有パスは`server/internal/match/**`だけなので、`server/main.go`への登録もHTTP/WebSocketの実装も行わない。#7が呼べるservice境界を公開する。

## Goals / Non-Goals

**Goals:**

- 二人を結びつけ、サーバーを盤面の正本として確定stateを両者へ届ける。
- 重複と競合の判定をルール側の`expectedTurn`に委ね、transport側に状態を持たない。
- 切断してもroomとchannelが残らないようにする。
- transportを持ち込まずにpackage testで全経路を確認できるようにする。

**Non-Goals:**

- transportの実装、永続化、棋譜。
- ルール判断の複製。

## Decisions

### 配信はchannelで行い、transportを持ち込まない

参加者への配信は`<-chan Event`で渡す。WebSocketもHTTPも知らないので、#7はchannelを読んで好きな方式へ流せる。testも実接続なしで配信内容を確認できる。

### 配信が詰まったら捨てる

各席のchannelはバッファ付きで、満杯なら送信を諦める。契約が「取りこぼしても次の受理で追いつく」「欠落検出の機構を設けてはならない」と定めているため、ブロックして対局を止めるより捨てる方が正しい。state全体を毎回送るので、次の1通で追いつく。

### 重複排除の状態を持たない

`commandId`は記憶せず、受理の可否は`expectedTurn`だけで決まる。判定はルールengineが行い、match側は結果を配信するだけにする。同じ`commandId`でも`expectedTurn`が合えば受理し、違う`commandId`でも古ければ`stale_turn`になる。testでは両方向を確認する。

### 拒否は送信者にだけ返す

受理したcommandだけをroomへ配信し、拒否は送信者へのresponseに留める。相手の画面が拒否で乱れないようにするためで、契約の「拒否されたcommandはroomへ配信しない」に対応する。拒否responseには変更前stateを載せ、クライアントが`stale_turn`を再同期として扱えるようにする。

### 席を名乗るcommandを検証する

`command.Player`が送信者の席と食い違う場合は`not_your_turn`で拒否する。ルールengineは`currentPlayer`しか見ないため、相手の手番に相手を騙って送るcommandはルール側では通ってしまう。誰がどの席かはmatch側しか知らないので、ここで止める。

### 切断は席を残したまま接続だけ落とす

切断ではplayerIDと席をroomに残し、channelだけ閉じる。同じplayerIDが再接続したら新しいchannelを渡し、現在state全体を1通配信して復帰させる。契約の「再接続直後にも現在stateを1通配信する」に対応する。

roomのphaseは、片方が切断している間`suspended`、両方揃えば`playing`へ戻す。二人とも切断したらroomを閉じ、Managerのmapからも消す。channelはすべてcloseするので、goroutineもroomも残らない。

### seedを差し替え可能にする

`WithSeed`で全roomのseedを固定できる。testで対局を決定的にするためで、既定は乱数。

## Risks / Trade-offs

- [配信を捨てる設計なので、遅い受信者は中間のeventを見落とす] → 契約どおりの挙動で、state全体が毎回届くため表示は次の受理で追いつく。eventの履歴が要る用途は棋譜（#6）が担う。
- [room単位のmutexで直列化するため、同じroomのcommandは並行に評価されない] → ターン制なので同時に進む手は1つだけで、直列化が正しい。room間は独立に動く。
- [再接続を`playerID`の一致だけで認めるため、なりすましを防げない] → P0のローカル対戦では認証を持たない。認証が入る段階で参加時のtokenへ差し替える。
- [切断の検出はtransport側の責務なので、このpackageは`Leave`が呼ばれるまで気づけない] → #7が接続断で`Leave`を呼ぶ。タイムアウトによる自動終了は、必要になった時点でManagerへ足せる。
