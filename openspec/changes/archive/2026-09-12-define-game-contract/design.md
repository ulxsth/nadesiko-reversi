## Context

動機は`proposal.md`を参照。現状はブラウザの盤面描画とgonako起動スモークだけがあり、ルール、runtime、transportで共有する型は存在しない。元実装の振る舞いには、固定端色の扱い、浮動小数による勝敗境界、盤面未充足時の終了に曖昧さがある。

## Goals / Non-Goals

**Goals:**

- 後続Agentが別々の言語で同じ結果を実装できる精度の契約を作る。
- JSON Schemaで境界を機械検証可能にする。
- seedとcommand列だけで対局を再現可能にする。
- Issue #1の専有パス`docs/contracts/**`と`openspec/changes/define-game-contract/**`だけで完結する。

**Non-Goals:**

- API handler、gonako呼び出し、UI、永続化の実装。
- WebSocket serverの選定やAWS構成の再現。
- 暗号学的乱数や改ざん防止署名。

## Decisions

### 盤面は64要素row-major配列とする

なでしこ、Go、JSON Schemaのすべてで扱いやすく、8×8の二重配列で発生しやすい可変長rowを排除できる。可読性の高い二重配列も検討したが、index=`row*8+col`を規約化する方が検証が単純になる。

### 元実装の挟み規則を明文化し、固定端の参照バグは修正する

色の陣営判定を行わず「2個以上の連続駒」を挟み成立とする独自性は維持する。一方、固定端`a2`は最遠の駒の着手前色と定義する。元実装の一部は変換対象の末尾を`a2`としておりコメントと不一致なため、その挙動は互換対象にしない。

### 乱数は32bit LCGで契約化する

Go標準乱数やブラウザ乱数は実装・versionで系列が変わり得る。整数演算だけのLCGを採用し、seedとcommand列から棋譜を決定的に再生する。乱数品質より再現性を優先する。

### 拒否されたcommandは完全に原子的とする

盤面だけでなくturnと乱数状態も変更しない。クライアント再送や競合で結果が変わらないよう、`expectedTurn`と`commandId`を境界に含める。

### Schemaは責務別の複数ファイルに分ける

`game-state`、`runtime-request`、`runtime-response`、`game-record`を別ファイルにし、`$ref`で共有定義を参照する。巨大なoneOf一枚より後続packageの所有境界が明確になる。

## Risks / Trade-offs

- [JavaScriptのnumberで32bit乗算が不正確になる] → 各実装は32bit整数への正規化を行い、固定vectorで一致を検証する。
- [独自の挟み規則が通常のリバーシと違い利用者が戸惑う] → 人間向け仕様とUI説明に「色で敵味方を判定しない」と明記する。
- [JSON Schemaだけでは意味的制約を表現し切れない] → Schemaに加え、正常・境界・失敗のcanonical exampleを置く。
- [将来のprotocol変更] → `version`を必須にし、未知versionを明示拒否する。

## Migration Plan

この契約は新規導入であり既存データ移行はない。#1マージ後にchangeをarchiveし、後続Issueは生成されたmain specを参照する。契約が意図と違う場合は実装前に新しいOpenSpec changeで更新する。
