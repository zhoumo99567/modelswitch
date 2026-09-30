#!/bin/zsh
set -euo pipefail

ROOT="${0:A:h}/.."
cd "$ROOT"
VERSION="${1:-$(tr -d '[:space:]' < VERSION)}"
if [[ ! "$VERSION" =~ '^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$' ]]; then
  echo "Version must use semantic versioning, for example 0.2.0" >&2
  exit 1
fi

OUT="release/$VERSION"
mkdir -p "$OUT"
NAME="ModelSwitcher-$VERSION-macos-universal.app.zip"
ZIP="$OUT/$NAME"
BASE_URL="${MODELSWITCHER_UPDATE_BASE_URL:-}"

wails build -clean -platform darwin/universal -ldflags "-X main.AppVersion=$VERSION"
ditto -c -k --keepParent build/bin/model-switcher.app "$ZIP"
SHA256="$(shasum -a 256 "$ZIP" | awk '{print $1}')"
URL=""
if [[ -n "$BASE_URL" ]]; then URL="${BASE_URL%/}/$VERSION/$NAME"; fi

python3 - "$OUT/latest.json" "$VERSION" "$URL" "$SHA256" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
data = {}
if path.exists():
    try: data = json.loads(path.read_text())
    except Exception: data = {}
data["version"] = sys.argv[2]
data.setdefault("windows", {"url": "", "sha256": ""})
data["macos"] = {"url": sys.argv[3], "sha256": sys.argv[4]}
path.write_text(json.dumps(data, indent=2) + "\n")
PY

if [[ "${PUBLISH:-0}" == "1" ]]; then
  : "${MODELSWITCHER_S3_URI:?Set MODELSWITCHER_S3_URI before publishing}"
  aws s3 cp "$ZIP" "${MODELSWITCHER_S3_URI%/}/$VERSION/$NAME" --only-show-errors
  aws s3 cp "$OUT/latest.json" "${MODELSWITCHER_S3_URI%/}/latest.json" --only-show-errors --content-type application/json
fi

echo "Created $ZIP"
echo "Created $OUT/latest.json"
