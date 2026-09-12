#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

test -x "$ROOT_DIR/.tools/go/bin/go"
test -x "$ROOT_DIR/.tools/bin/gonako"
test -x "$ROOT_DIR/.tools/node/bin/node"
test -x "$ROOT_DIR/.tools/openspec/node_modules/.bin/openspec"
test -s "$ROOT_DIR/web/vendor/wnako3.js"

export PATH="$ROOT_DIR/.tools/go/bin:$ROOT_DIR/.tools/node/bin:$ROOT_DIR/.tools/openspec/node_modules/.bin:$ROOT_DIR/.tools/bin:$PATH"

actual="$($ROOT_DIR/.tools/bin/gonako "$ROOT_DIR/rules/smoke.nako3" | tr -d '\r')"
if [[ "$actual" != "gonako-ready" ]]; then
  echo "gonako smoke失敗: $actual" >&2
  exit 1
fi

"$ROOT_DIR/.tools/go/bin/go" test ./...
openspec validate --all --strict

echo "check完了"
