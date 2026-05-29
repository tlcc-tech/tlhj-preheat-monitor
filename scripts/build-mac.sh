#!/usr/bin/env bash
set -euo pipefail

VERSION="$(sed -nE 's/.*"productVersion"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' wails.json | head -n 1)"
if [[ -z "${VERSION}" ]]; then
  VERSION="dev"
fi

wails build -platform darwin/universal -clean -ldflags "-X main.AppVersion=${VERSION}"

APP_NAME="新服预约监控"
APP_PATH="build/bin/${APP_NAME}.app"
DMG_PATH="build/bin/${APP_NAME}.dmg"

if command -v create-dmg >/dev/null 2>&1; then
  rm -rf build/dmg
  mkdir -p build/dmg
  cp -R "${APP_PATH}" build/dmg/
  create-dmg --volname "${APP_NAME}" \
    --volicon "${APP_PATH}/Contents/Resources/iconfile.icns" \
    --app-drop-link 300 180 \
    "${DMG_PATH}" \
    build/dmg
  echo "DMG created: ${DMG_PATH}"
else
  echo "create-dmg not found; skipped dmg packaging. Install with: brew install create-dmg"
fi

echo "APP built: ${APP_PATH}"
