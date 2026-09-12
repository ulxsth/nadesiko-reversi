## Context

`openspec/specs/runtime-protocol/spec.md`が外部挙動の正本で、`docs/contracts/*.schema.json`がJSONの正本である。#2のルールengineは`gonako rules/game/main.nako3`を1回起動し、stdinのJSON object 1件を評価してstdoutへJSON object 1件を返す。壊れたJSONを渡した場合は非0終了とstderr診断になり、responseは出ない。

この境界は#5、#6、#7が共有する。Issue #3の専有パスは`server/internal/runtime/**`と`server/internal/protocol/**`だけなので、`server/main.go`への登録は行わず、packageとして完結させる。

## Goals / Non-Goals

**Goals:**

- 契約のJSONをGoの型で往復でき、契約exampleと一致することを検証する。
- gonakoの起動方法を1か所へ閉じ、呼び出し側がprocessを意識しないようにする。
- 実行基盤の失敗をゲーム上の拒否と区別し、型で判別できるようにする。
- 同時呼び出しが互いの入出力を壊さないことをtestで示す。

**Non-Goals:**

- transportへの登録、対局の保持、永続化。
- ルール判断のGoへの複製。

## Decisions

### Runner interfaceをEvaluate 1メソッドにする

`Evaluate(ctx, protocol.Request) (*protocol.Response, error)`だけを境界にする。newGameとapplyCommandはpackage関数`NewGame`、`ApplyCommand`として提供し、interfaceの実装点を増やさない。差し替え実装が1メソッドで済むため、#5のhandler testが軽くなる。

### ゲーム上の拒否はerrorではなくResponseで返す

`occupied`や`not_your_turn`は契約上の正常な応答であり、障害ではない。これらは`Response.OK=false`として返し、errorはnilにする。errorが返るのはtimeout、異常終了、出力の解釈失敗だけにする。呼び出し側は「errorなら基盤の問題、OK=falseならplayerへ見せる日本語message」と一律に扱える。

### 不正requestはprocessを起動せず拒否する

`Request.Validate()`が契約違反を検出した場合、gonakoを起動せずに同じcodeの拒否responseを返す。盤外座標や未対応versionのためにprocessを起動する必要はなく、engineが返す結果と区別もつかない。testでは失敗するstubをルールsourceに差し替え、拒否responseが返ること自体で未起動を確認する。

### 一時ファイルを使わずpipeだけで完結させる

requestは`bytes.Reader`としてstdinへ、responseは`bytes.Buffer`としてstdoutから受け取る。共有する一時ファイルも作業ディレクトリも持たないため、同時呼び出しが混線しない。`Gonako`が保持するのは不変の構成だけなので、複数goroutineから同じrunnerを使える。

### 実行基盤の失敗を3種類の型へ分ける

`TimeoutError`は制限時間超過、`ExecError`は非0終了とstderr診断、`DecodeError`は出力が契約のresponseでない場合を表す。`ExecError.ProtocolError()`は契約の`invalid_json`へ写し、engineがresponseを作れなかった場合でもclientへ返すcodeを決められるようにする。契約違反のresponseは`DecodeError`で包み、理由の`*protocol.Error`を`errors.As`で取り出せるようにする。

### 差し替え実装を別packageへ置く

`runtimetest.Fake`は`_test.go`ではなく`server/internal/runtime/runtimetest`に置く。#5と#7のhandler testから参照できるようにするためで、`net/http/httptest`と同じ形にする。Fakeはrequestを記録し、応答関数で差し替えられ、複数goroutineから同時に使える。

### 契約exampleをtestdataへ複製せず参照する

往復testは`docs/contracts/examples/*.json`を相対パスで読む。契約が正本なので、複製すると更新時に二重管理になる。読み取りだけなので他Issueの専有パスを変更しない。

## Risks / Trade-offs

- [1評価ごとにprocessを起動するため、着手のたびに起動コストがかかる] → P0のローカル対戦では1手あたり数十msで足り、常駐process化は測定してから検討する。境界がinterfaceなので実装差し替えで対応できる。
- [gonakoの診断文言に依存するとengine更新で壊れる] → testでは終了コードとstderrの有無だけを見て、文言そのものは判定に使わない。
- [`ExecError`を一律に`invalid_json`へ写すと、engine内部の別の障害も同じcodeになる] → 契約が定義するcodeに他の選択肢がないため許容し、原因の識別はGo側のerror型で行う。
- [packageの名前が標準ライブラリの`runtime`と衝突する] → `architecture.md`が定めた配置に合わせ、import側でaliasを使う余地を残す。
