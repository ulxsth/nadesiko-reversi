## Why

元実装にはグラデーションリバーシの核となる挙動が複数のGo/Wasm/Lambda実装へ分散し、挟み判定や終了条件にも曖昧さがある。ローカルプレイMVPを複数Agentで並行実装する前に、ゲーム規則とブラウザ・Go・gonako間の契約を単一の仕様として固定する。

## What Changes

- 8×8盤面、座標系、初期配置、合法手、色変換、手番、パス、終了、勝敗を定義する。
- seed付き決定的な次色生成を定義し、同じ入力から同じ対局を再現可能にする。
- game state、command、runtime request/response、対局記録のJSON契約を定義する。
- 不正入力を安定したerror codeへ写像する。
- 元実装をそのまま踏襲する点と、MVPのため意図的に修正・後回しにする点を記録する。

### Non-goals

- ゲームエンジン、Go adapter、ブラウザUIそのものは実装しない。
- WebSocket、マッチメイキング、永続化、認証の具体的なtransportは定義しない。
- 元実装のAWS構成やWhitespace変換APIは移植しない。

## Capabilities

### New Capabilities

- `gradient-reversi-rules`: グラデーションリバーシの盤面遷移、合法手、決定的な色生成、終了と勝敗。
- `runtime-protocol`: ブラウザ、Go、gonakoおよび将来の対局記録が共有するversioned JSON契約とerror code。

### Modified Capabilities

- なし。

## Impact

- GitHub Issue: #1
- Branch: `feature/define-game-contract`
- Dependencies: なし
- Exclusively owned paths: `docs/contracts/**`, `openspec/changes/define-game-contract/**`
- 後続の#2、#3、#4およびローカルMVP統合Issueは、この契約を実装上の正本として参照する。
