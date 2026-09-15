## Why

#6 / PR #28 の棋譜再生は、安全な暫定構成として読み取り後に`NewGame`と各手の`ApplyCommand`をGo側から呼ぶ。このため60手の棋譜1本で約62回gonakoを起動する。ECS/FargateやLambdaへ移すと、1リクエストがプロセス62個に化ける。

原因は`rules/game/main.nako3`が末尾で無条件にCLIを実行することにあった。再生ハーネスから取り込めないため、ルールの適用をGo側へ出すしかなかった。ルール本体をCLIエントリポイントから分離すれば、読み取りから全手の適用までを1プロセスに畳める。

あわせて、棋譜が前提とするルール版を棋譜自身へ持たせる。ルールの振る舞いが変わった後に古い棋譜を黙って誤再生すると、再現性という棋譜の存在理由が崩れる。

## What Changes

- ゲームルール本体を`rules/game/rules.nako3`へ分離し、`rules/game/main.nako3`はそれを取り込むCLIエントリポイントにする。標準入力JSONと`--test`の挙動は変えない。
- 再生ハーネスがルール本体を取り込み、棋譜の読み取り・全手の適用・各手の盤面生成・最終盤面と勝者の算出をgonako 1プロセスで完結させる。
- 棋譜へ`ルール版宣言`行を必須項目として追加し、未対応の版はgonako起動前に行番号つきで拒否する。
- 許可語彙の外にある行を実行前に拒否することを契約として明記し、棋譜を任意コード実行経路にしない。
- `docs/contracts/game-record.md`を新設し、実行可能な日本語表現、JSON Schema、許可語彙、版互換性を一致させる。
- ルールファイルは読み取り専用配置で動作させ、書き込みはOSの一時領域だけに限り、実行後に必ず削除する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `runtime-protocol`: 対局記録へルール版を加え、棋譜の許可語彙、再生の実行モデル、一時領域の扱いを要件として追加する。

## Impact

- GitHub Issue: #30
- Branch: `feature/single-process-replay`
- Dependencies: #6（`develop`へマージ済み）
- Exclusively owned paths: `rules/game/**`、`rules/replay/**`、`server/internal/replay/**`、`docs/contracts/game-record.md`、`docs/contracts/game-record.schema.json`、`openspec/changes/single-process-replay/**`
- `replay.NewReplayer`は`runtime.Runner`を取らなくなる。まだ`server/main.go`へ配線されていないため、影響は本package内に閉じる。
- #7は再生の配線時にrule runnerを渡す必要がない。

## Non-goals

- 常駐gonako daemon、プロセスプール、ワーカーの導入は行わない。1棋譜を1回のone-shotプロセスで処理する。
- 棋譜と対局状態の永続化は扱わない。保存interfaceの差し替えで対応する。
- HTTP routeやWebSocketへの登録は行わない。`server/main.go`は#7が配線する。
- `rules/game`のルールそのものの振る舞いは変えない。分離だけを行う。
- `openspec/specs/**`は編集しない。コーディネータがマージ後のarchiveで反映する。
