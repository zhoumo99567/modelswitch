#!/bin/zsh
set -euo pipefail

ROOT="${0:A:h}/.."
cd "$ROOT"
VERSION="${1:-$(tr -d '[:space:]' < VERSION)}"
node scripts/release-manifest.mjs prepare "$VERSION"

OUT="release/$VERSION"
mkdir -p "$OUT"
NAME="ModelSwitcher-$VERSION-macos-universal.app.zip"
ZIP="$OUT/$NAME"
BASE_URL="${MODELSWITCHER_UPDATE_BASE_URL:-}"

LDFLAGS="-X main.AppVersion=$VERSION"
if [[ -n "${MODELSWITCHER_BUILD_UPDATE_URL:-}" ]]; then
  if [[ "$MODELSWITCHER_BUILD_UPDATE_URL" != https://* || "$MODELSWITCHER_BUILD_UPDATE_URL" == *[[:space:]]* || "$MODELSWITCHER_BUILD_UPDATE_URL" == *"'"* ]]; then
    echo "Build update URL must be HTTPS without spaces or quotes" >&2
    exit 1
  fi
  LDFLAGS="$LDFLAGS -X main.defaultUpdateManifestURL=$MODELSWITCHER_BUILD_UPDATE_URL"
fi
wails build -clean -platform darwin/universal -ldflags "$LDFLAGS"
BUNDLE="build/bin/$(node -p 'require("./wails.json").name').app"
EXECUTABLE="$BUNDLE/Contents/MacOS/$(node -p 'require("./wails.json").outputfilename')"
BUILT_VERSION="$("$EXECUTABLE" --version)"
if [[ "$BUILT_VERSION" != "$VERSION" ]]; then
  echo "Built application version does not match VERSION: $BUILT_VERSION" >&2
  exit 1
fi
ditto -c -k --keepParent "$BUNDLE" "$ZIP"
DOWNLOAD_BASE="${MODELSWITCHER_RELEASE_DOWNLOAD_BASE_URL:-}"
if [[ -z "$DOWNLOAD_BASE" && -n "$BASE_URL" ]]; then DOWNLOAD_BASE="${BASE_URL%/}/$VERSION"; fi
if [[ "${PUBLISH:-0}" == "1" && -z "$BASE_URL" ]]; then
  echo "Set MODELSWITCHER_UPDATE_BASE_URL before S3 publishing" >&2
  exit 1
fi
MODELSWITCHER_RELEASE_DOWNLOAD_BASE_URL="$DOWNLOAD_BASE" node scripts/release-manifest.mjs manifest "$VERSION" "$OUT"

if [[ "${PUBLISH:-0}" == "1" ]]; then
  : "${MODELSWITCHER_S3_URI:?Set MODELSWITCHER_S3_URI before publishing}"
  aws s3 cp "$ZIP" "${MODELSWITCHER_S3_URI%/}/$VERSION/$NAME" --only-show-errors
  aws s3 cp "$OUT/latest.json" "${MODELSWITCHER_S3_URI%/}/latest.json" --only-show-errors --content-type application/json
  aws s3 cp "$OUT/SHA256SUMS.txt" "${MODELSWITCHER_S3_URI%/}/$VERSION/SHA256SUMS.txt" --only-show-errors
fi

echo "Created $ZIP"
echo "Created $OUT/latest.json"
