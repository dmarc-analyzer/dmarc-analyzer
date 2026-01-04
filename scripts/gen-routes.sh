#!/usr/bin/env bash
set -euo pipefail

if ! command -v openapi-generator >/dev/null 2>&1; then
  echo "openapi-generator is not installed. Run: brew install openapi-generator" >&2
  exit 1
fi

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
SPEC_PATH="$ROOT_DIR/api/openapi.json"
TEMPLATE_DIR="$ROOT_DIR/api/openapi-generator"
OUT_DIR=$(mktemp -d)

cleanup() {
  rm -rf "$OUT_DIR"
}
trap cleanup EXIT

openapi-generator generate \
  -g go-gin-server \
  -i "$SPEC_PATH" \
  -o "$OUT_DIR" \
  -t "$TEMPLATE_DIR" \
  --global-property=apis,models=false,supportingFiles=routers.go \
  --additional-properties=packageName=handler,apiPath=go

cp "$OUT_DIR/go/routers.go" "$ROOT_DIR/backend/handler/routes.gen.go"
cp "$OUT_DIR/go/api_default.go" "$ROOT_DIR/backend/handler/handlers.gen.go"
gofmt -w "$ROOT_DIR/backend/handler/routes.gen.go" "$ROOT_DIR/backend/handler/handlers.gen.go"
