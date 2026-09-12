# 開発フロー

## ブランチ構成

```text
main
└── develop
    ├── feature/define-game-contract
    ├── feature/implement-game-rules
    └── feature/...
```

- `main`: デモ可能な状態だけを置くリリースブランチ
- `develop`: feature PRの統合先兼デフォルトブランチ
- `feature/<change-id>`: Issue 1件とOpenSpec change 1件に対応する作業ブランチ

featureブランチは最新の`origin/develop`から作り、PRは`develop`へ送ります。`main`への変更は`develop`からのリリースPRだけにします。

## 競合を避けるルール

担当Issueの「専有パス」は、そのIssueの担当者だけが変更できます。依存関係のないIssueは、専有パスが交差しないように分けています。共有エントリポイントへの配線は、各コンポーネントのマージ後に統合Issueで行います。

担当外の変更が必要になった場合は、その場で編集せず次のどちらかにします。

1. 依存Issueの担当者へインターフェース変更を依頼する。
2. 統合Issueへ追記し、現在のPRでは未配線の実装とテストまでに留める。

## OpenSpecの流れ

```text
Issue作成
  → feature/<change-id>
  → openspec/changes/<change-id>/ を作成
  → 仕様レビュー
  → 実装と検証
  → developへPR・マージ
  → コーディネータがchangeをarchive
```

feature PR内でarchiveすると、複数PRが共有する`openspec/specs/**`で競合します。そのためarchiveは必ずマージ後に一件ずつ行います。

## ローカル検証

PowerShell:

```powershell
.\scripts\wsl.ps1 bootstrap
.\scripts\wsl.ps1 check
```

WSL:

```bash
make bootstrap
make check
```

`make check`はGoサーバーのbuild/test、gonakoのスモーク実行、OpenSpecのstrict validationを行います。

## PRの条件

- baseが`develop`である
- IssueとOpenSpec changeとブランチの`change-id`が一致する
- 専有パス外の変更を含まない
- `make check`が成功している
- Issueの受け入れ条件をPR本文で確認している
- archiveしていない
