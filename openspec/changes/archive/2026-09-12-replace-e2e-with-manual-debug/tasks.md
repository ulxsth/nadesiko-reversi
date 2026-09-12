## 1. 自動テスト範囲

- [x] 1.1 実serverとgonakoを起動する`tests/e2e`を削除する
- [x] 1.2 ルール、protocol、runtime、localgameのpackage testを維持する

## 2. 手動デバッグ

- [x] 2.1 起動、新規対局、合法手、不正手、seed再現の確認手順を文書化する
- [x] 2.2 パス、終局、勝者表示の確認手順を文書化する
- [x] 2.3 実施環境、commit、PASS/FAIL、失敗時情報を記録できる形式にする

## 3. 後続作業との整合

- [x] 3.1 アーキテクチャとP0バックログから自動E2E前提を外す
- [x] 3.2 #7のIssueテンプレートを手動デバッグ完了条件へ更新する
- [x] 3.3 実Issue #7の本文を同じ完了条件へ更新する
- [x] 3.4 WSLで`make check`とOpenSpec strict validationを成功させる
