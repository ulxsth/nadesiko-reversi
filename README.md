# なでしこ・グラデーションリバーシ

「アンミカリバーシ」の体験を、日本語プログラミング言語なでしこで再構成するプロジェクトです。

- ブラウザ: なでしこ3（Canvas、入力、将来のWebSocket通信）
- サーバー: Go
- ルール実行: gonako
- 開発環境: WSL2 Ubuntu 24.04

現在は実行環境と最初の縦切りを用意しています。ブラウザで盤面を描画し、クリックで駒を置けます。サーバーからgonakoのスモークプログラムも実行できます。

## 最短で起動

PowerShellから次を実行します。

```powershell
.\scripts\wsl.ps1 bootstrap
.\scripts\wsl.ps1 check
.\scripts\wsl.ps1 dev
```

その後、ブラウザで <http://127.0.0.1:4173> を開きます。

`dev` はフォアグラウンドで動きます。終了は `Ctrl+C` です。

このPCのWSLログインシェルには、存在しない `$HOME/.rye/env` を読む設定が残っています。`scripts/wsl.ps1` は `bash --noprofile --norc` を使うため、その設定に影響されず起動できます。

## WSLから直接実行

このディレクトリをWSLで開いて、次を実行します。

```bash
make bootstrap
make check
make dev
```

bootstrapは依存をプロジェクト内の `.tools/` にだけ配置します。WSLのシステム領域やホームディレクトリにはインストールしません。

固定しているバージョン:

- Go 1.26.0
- gonako 3.8.4
- なでしこ3ブラウザランタイム 3.8.1

## エンドポイント

- `GET /healthz`: Goサーバーの状態
- `GET /api/runtime/smoke`: サーバーからgonakoを実行
- `/`: なでしこ3の盤面プロトタイプ

## ディレクトリ

```text
rules/              gonakoで動かす日本語ルール
server/             Go製HTTP/WebSocketサーバー（現在はHTTPの土台）
web/                ブラウザ版なでしこ3クライアント
scripts/            WSL用bootstrap・検証・起動ラッパー
```

## 次の実装

1. 変則リバーシの合法手・色変化・勝敗判定を `rules/` に実装する
2. Goサーバーから盤面JSONをgonakoへ渡す
3. WebSocketでサーバー確定盤面を配信する
4. 対局ログを実行可能な日本語コードとして保存・再生する
