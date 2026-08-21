#!/usr/bin/env bash
# macos-bundle.sh — wrap the blinkybeacon-tray binary in a minimal .app bundle.
#
# A bare Mach-O binary works (double-click opens a Terminal window and the menu
# bar icon appears), but a bundle is what a Mac user expects: no Terminal, no
# Dock icon (LSUIElement), and something that can be dropped in /Applications.
#
#   scripts/macos-bundle.sh <binary> <out.app>
set -euo pipefail
bin="${1:?usage: macos-bundle.sh <binary> <out.app>}"
app="${2:?usage: macos-bundle.sh <binary> <out.app>}"
name="BlinkyBeacon"
exe="blinkybeacon-tray"

rm -rf "$app"
mkdir -p "$app/Contents/MacOS"
cp "$bin" "$app/Contents/MacOS/$exe"
chmod 755 "$app/Contents/MacOS/$exe"
cat > "$app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>$name</string>
	<key>CFBundleDisplayName</key>
	<string>$name</string>
	<key>CFBundleIdentifier</key>
	<string>com.blinkybeacon.tray</string>
	<key>CFBundleExecutable</key>
	<string>$exe</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleVersion</key>
	<string>${BUNDLE_VERSION:-0}</string>
	<key>CFBundleShortVersionString</key>
	<string>${BUNDLE_VERSION:-0}</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>LSUIElement</key>
	<true/>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
PLIST
echo "bundled $app"
