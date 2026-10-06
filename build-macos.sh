#!/bin/zsh
set -euo pipefail

# Run this script on macOS. Wails cannot link the WebKit-based app bundle on Windows.
VERSION="${1:-$(tr -d '[:space:]' < VERSION)}"
npm --prefix frontend install
wails build -clean -platform darwin/universal -ldflags "-X main.AppVersion=$VERSION"

echo "Built build/bin/ModelSwitcher.app version $VERSION"
