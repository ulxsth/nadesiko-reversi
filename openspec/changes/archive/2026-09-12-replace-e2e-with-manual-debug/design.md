# Design

## 検証レイヤー

ルールや状態遷移の決定論的な確認は、既存のなでしこself-testとGo package testで自動化を維持する。process起動、HTTP、ブラウザまでを横断する確認だけを手動デバッグへ移す。

## 手動手順の役割

チェックリストは単なる「触って確認」ではなく、対象commit、環境、各観点のPASS/FAIL、失敗時のrequest/responseを記録できる形式にする。これにより自動E2Eの保守コストを持たず、ハッカソン前や統合PRで同じ観点を再実行できる。

## CIへの影響

`scripts/check-wsl.sh`は既に`go test ./...`を実行するため、`tests/e2e`の削除だけで実runtime E2Eは対象外になる。Makefileやworkflowの条件分岐は追加しない。
