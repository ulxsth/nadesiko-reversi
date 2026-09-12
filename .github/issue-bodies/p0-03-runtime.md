## ゴール

Goサーバーから#2のなでしこルールを安全に呼び出すruntime adapterと、後続のHTTP/WebSocket実装が共有するprotocol型を作る。

## Agent assignment contract

- change-id: `integrate-gonako-runtime`
- branch: `feature/integrate-gonako-runtime`
- 専有パス:
  - `server/internal/runtime/**`
  - `server/internal/protocol/**`
  - `openspec/changes/integrate-gonako-runtime/**`
- 変更禁止:
  - `server/main.go`
  - `rules/**`
  - `web/**`
  - `openspec/specs/**`
  - `go.mod`を含む共有設定ファイル
- 依存Issue: #1、#2（ともに`develop`へマージ済みであること）

## 受け入れ条件

- [ ] #1のJSON型をGoで表現し、encode/decodeの往復テストがある
- [ ] gonakoのパス、ルールファイル、stdin/stdoutをruntime adapterへ閉じ込める
- [ ] 実行timeout、終了コード、stderr、壊れたJSONを型付きエラーに変換する
- [ ] 同時呼び出しで一時ファイルや入力が混線しない
- [ ] #2の代表ケースをGoテストから実行できる
- [ ] handlerからmock可能なインターフェースになっている
- [ ] OpenSpecの全artifactが揃い、WSLで`make check`が成功する

## マージ条件

package単位でテスト可能にし、HTTP routeや`server/main.go`への登録は#7へ残す。feature PRではarchiveしない。
