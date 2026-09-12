## Context

`runtime.Runner`は完全stateを含むrequestをgonakoへ渡す。ローカルMVPでは1サーバーにつき1対局だけをGoメモリへ保持し、ブラウザは確定responseを描画する。

## Goals / Non-Goals

**Goals:**

- `POST /api/local/runtime`でnewGameとapplyCommandを扱う。
- applyCommandではサーバー保持stateを正本にし、clientのstate改変をルール評価へ渡さない。
- 成功commandだけを保持stateへ反映し、拒否時は同じstateを返す。
- 一つのブラウザを交互に操作して終了まで進められる。

**Non-Goals:**

- 複数対局、認証、再接続、履歴保存、WebSocket。

## Decisions

### ローカル対局serviceがstateを直列化する

`localgame.Service`はmutexでnewGameとcommandを直列化し、成功responseのstateだけを保持する。runtime process実行中もlockを維持することで、同じturnへの二重commandが同時に評価されない。

### transportはruntime envelopeを再利用する

browserは`runtime-protocol`のrequestをそのまま1 endpointへPOSTする。新しいJSON形を増やさず、gonakoの成功・拒否responseをそのまま描画層へ渡す。

### hot-seatでは確定stateのcurrentPlayerを操作playerにする

認証のない1ブラウザ二人用なので、response受信後に操作playerを`currentPlayer`へ合わせる。手番ごとにselectを切り替える必要がなく、サーバーの手番検証は残る。

### E2Eは実server processと実gonakoを使う

一時portでserver binaryを起動し、HTTPで不正着手、正常着手、reset、終了までのcommand列を流す。Go内にルール期待値を複製せず、state遷移と終了条件だけを観測する。

## Risks / Trade-offs

- [server再起動で対局が消える] → ローカルMVPの明示的制約とし、永続化は後続Issueへ分離する。
- [process起動中に次commandが待つ] → 1対局の順序保証として許容する。複数room化時はroom単位lockへ変更する。
- [ブラウザから完全stateが送られる] → serviceが保持stateへ置換してからruntimeを呼び、client stateを信用しない。
