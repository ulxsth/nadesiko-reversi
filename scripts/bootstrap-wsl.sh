#!/usr/bin/env bash
set -euo pipefail

GO_VERSION="1.26.0"
GONAKO_VERSION="3.8.4"
NADESIKO_VERSION="3.8.1"

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLS_DIR="$ROOT_DIR/.tools"
BIN_DIR="$TOOLS_DIR/bin"
VENDOR_DIR="$ROOT_DIR/web/vendor"

mkdir -p "$BIN_DIR" "$VENDOR_DIR"

if [[ ! -x "$TOOLS_DIR/go/bin/go" ]]; then
  archive="$TOOLS_DIR/go-${GO_VERSION}.linux-amd64.tar.gz"
  echo "Go ${GO_VERSION} を取得しています..."
  curl -fL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o "$archive"
  tar -xzf "$archive" -C "$TOOLS_DIR"
  rm -f "$archive"
fi

if [[ ! -x "$BIN_DIR/gonako" ]]; then
  archive="$TOOLS_DIR/gonako-${GONAKO_VERSION}-linux-amd64.zip"
  extract_dir="$(mktemp -d "$TOOLS_DIR/gonako.XXXXXX")"
  echo "gonako ${GONAKO_VERSION} を取得しています..."
  curl -fL "https://github.com/kujirahand/nadesiko3go/releases/download/${GONAKO_VERSION}/gonako-${GONAKO_VERSION}-linux-amd64.zip" -o "$archive"
  unzip -q "$archive" -d "$extract_dir"
  install -m 0755 "$extract_dir/gonako" "$BIN_DIR/gonako"
  rm -f "$archive"
  rm -r "$extract_dir"
fi

if [[ ! -s "$VENDOR_DIR/wnako3.js" ]]; then
  echo "なでしこ3ブラウザランタイム ${NADESIKO_VERSION} を取得しています..."
  curl -fL "https://cdn.jsdelivr.net/npm/nadesiko3@${NADESIKO_VERSION}/release/wnako3.js" -o "$VENDOR_DIR/wnako3.js"
fi

echo ""
"$TOOLS_DIR/go/bin/go" version
"$BIN_DIR/gonako" -e '「gonako-ready」と表示。'
echo "ブラウザランタイム: $(wc -c < "$VENDOR_DIR/wnako3.js") bytes"
echo "bootstrap完了"

