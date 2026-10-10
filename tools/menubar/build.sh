#!/bin/sh
# Build the menu bar launcher into tools/menubar/build/atc-scribe.app.
# With --install, also copy it to ~/Applications.
#
# Needs the Xcode command line tools (swiftc) and macOS 13 or later.
set -e
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
app="$here/build/atc-scribe.app"

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
sed "s|@REPO@|$repo|" "$here/AtcScribe.swift" > "$here/build/AtcScribe.swift"
swiftc -O -parse-as-library -target "$(uname -m)-apple-macos13.0" \
	-o "$app/Contents/MacOS/atc-scribe" "$here/build/AtcScribe.swift"
cat > "$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>atc-scribe</string>
	<key>CFBundleIdentifier</key><string>dev.atc-scribe.launcher</string>
	<key>CFBundleExecutable</key><string>atc-scribe</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>1.0</string>
	<key>LSMinimumSystemVersion</key><string>13.0</string>
	<key>LSUIElement</key><true/>
	<!-- Plain HTTP: the launcher only calls the addresses of its own settings,
	     all on the local network or the VPN (co-atc, the sidecar, radio-ctl), and
	     a name like macmini-fedora.lan is not among the local names ATS allows. -->
	<key>NSAppTransportSecurity</key>
	<dict><key>NSAllowsArbitraryLoads</key><true/></dict>
	<key>NSLocalNetworkUsageDescription</key>
	<string>co-atc, started from here, reads the receiving station's streams on the local network.</string>
</dict>
</plist>
EOF
codesign --force --sign - "$app" >/dev/null 2>&1 || true
echo "built $app"

if [ "$1" = "--install" ]; then
	mkdir -p "$HOME/Applications"
	rm -rf "$HOME/Applications/atc-scribe.app"
	cp -R "$app" "$HOME/Applications/"
	echo "installed $HOME/Applications/atc-scribe.app"
fi
