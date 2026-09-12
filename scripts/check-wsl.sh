#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

test -x "$ROOT_DIR/.tools/go/bin/go"
test -x "$ROOT_DIR/.tools/bin/gonako"
test -s "$ROOT_DIR/web/vendor/wnako3.js"

actual="$($ROOT_DIR/.tools/bin/gonako "$ROOT_DIR/rules/smoke.nako3" | tr -d '\r')"
if [[ "$actual" != "gonako-ready" ]]; then
  echo "gonako smoke失敗: $actual" >&2
  exit 1
fi

"$ROOT_DIR/.tools/go/bin/go" test ./...

echo "check完了"

