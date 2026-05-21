#!/usr/bin/env bash
# Refresh docs/screenshots/*.png by driving the running demo stack with
# shot-scraper inside the official Playwright Docker image.
#
# Why Docker: shot-scraper relies on Playwright, and Playwright's browser
# installer is OS-detection-strict (it doesn't recognize every Ubuntu
# release). Running it inside the official image sidesteps that.
#
# Prerequisites:
#   1. Demo stack must be up:
#        docker compose -f scripts/screenshots.compose.yml up -d
#      (wait ~10s for postgres init + server start)
#   2. Reachable at http://localhost:6767/
#
# Outputs:
#   docs/screenshots/domains.png
#   docs/screenshots/report-summary.png
#   docs/screenshots/report-detail.png

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLAYWRIGHT_IMAGE="${PLAYWRIGHT_IMAGE:-mcr.microsoft.com/playwright/python:v1.60.0-noble}"

# If host requires sudo for docker, set DOCKER_CMD=sudo docker
DOCKER_CMD="${DOCKER_CMD:-docker}"

# Smoke-test the stack before launching the screenshot container
if ! curl -sf -o /dev/null --max-time 5 http://localhost:6767/api/domains; then
  echo "ERROR: http://localhost:6767/api/domains is not reachable." >&2
  echo "       Bring the demo stack up first:" >&2
  echo "         $DOCKER_CMD compose -f scripts/screenshots.compose.yml up -d" >&2
  exit 1
fi

mkdir -p "$ROOT/docs/screenshots"

$DOCKER_CMD run --rm --network host \
  -v "$ROOT:/work" \
  -w /work \
  "$PLAYWRIGHT_IMAGE" \
  bash -c 'pip install -q shot-scraper && shot-scraper multi scripts/screenshots.yml'

# Docker writes as root; fix ownership back to the invoking user
if [ "$(id -u)" -ne 0 ]; then
  $DOCKER_CMD run --rm -v "$ROOT/docs/screenshots:/out" alpine \
    chown -R "$(id -u):$(id -g)" /out
fi

echo
echo "Done. Outputs:"
ls -la "$ROOT/docs/screenshots/"
