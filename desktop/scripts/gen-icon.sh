#!/bin/sh
# Regenerates the Bob Console app icon from build/icon.svg.
#
# Pipeline: headless Chrome renders the SVG to a 1024px transparent PNG, sips
# downsamples the Apple iconset sizes, and iconutil packs them into an icns.
# Requires macOS (sips + iconutil) and Google Chrome.
set -eu

cd "$(dirname "$0")/.."

CHROME="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"

"$CHROME" \
  --headless=new --disable-gpu --hide-scrollbars \
  --force-device-scale-factor=1 --window-size=1024,1024 \
  --default-background-color=00000000 \
  --screenshot=build/icon-1024.png \
  "file://$PWD/build/icon-render.html" >/dev/null 2>&1

mkdir -p build/icon.iconset
for spec in \
  "16 icon_16x16" "32 icon_16x16@2x" \
  "32 icon_32x32" "64 icon_32x32@2x" \
  "128 icon_128x128" "256 icon_128x128@2x" \
  "256 icon_256x256" "512 icon_256x256@2x" \
  "512 icon_512x512" "1024 icon_512x512@2x"; do
  set -- $spec
  sips -z "$1" "$1" build/icon-1024.png --out "build/icon.iconset/$2.png" >/dev/null 2>&1
done

iconutil -c icns build/icon.iconset -o build/icon.icns
cp build/icon.iconset/icon_512x512.png build/icon.png
cp build/icon.svg src/renderer/public/icon.svg

echo "wrote build/icon.icns, build/icon.png, src/renderer/public/icon.svg"
