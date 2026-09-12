## 1. 契約のGo表現

- [x] 1.1 request、response、state、command、eventの型とenumを定義し、`docs/contracts/examples/*.json`との往復で内容が変わらないことを確認する
- [x] 1.2 必須field、値域、phase依存条件、oneOfのvalidationを実装し、正常・不正・境界のtable testで各error codeを確認する
- [x] 1.3 契約の12件のerror codeを宣言し、件数と並びが`docs/contracts/protocol.md`の表と一致することを確認する

## 2. gonako runtime adapter

- [x] 2.1 `Runner` interfaceとgonako実装を作り、実行ファイルとルールsourceの構成errorを起動前に検出することを確認する
- [x] 2.2 stdin/stdoutのpipeだけで1回の評価を行い、seed=1のnewGameが契約の固定vector（nextColor=60、rngState=1015568748、合法手12件）と一致することを確認する
- [x] 2.3 timeout、非0終了とstderr、解釈できない出力、契約違反のresponseをそれぞれ専用のerror型へ変換し、`errors.As`で判別できることを確認する
- [x] 2.4 不正requestでprocessを起動せず拒否responseを返すことを、失敗するstubをルールsourceに差し替えて確認する

## 3. 後続Issueのための境界

- [x] 3.1 `runtimetest.Fake`を追加し、requestの記録・応答の差し替え・同時利用が動くことを確認する
- [x] 3.2 #2の代表ケース（着手の色変換、occupied、illegal_move、not_your_turn、stale_turn、pass_not_allowed）をGo testから実行し、拒否時にstateが変わらないことを確認する
- [x] 3.3 同時呼び出しの結果が直列実行と一致することを確認し、`go test -race`で共有状態の競合がないことを確認する

## 4. 検証

- [x] 4.1 `go vet ./server/...`と`go test -race ./server/...`が成功することを確認する
- [x] 4.2 WSLで`make check`を実行し、build・gonako smoke・Go test・`openspec validate --all --strict`が成功することを確認する
- [x] 4.3 Issue #3の受け入れ条件を一つずつ確認する
