#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
GO=${GO:-go}
os=$($GO env GOOS)
arch=$($GO env GOARCH)
tools_dir=${TOOLS_DIR:-"$root/tools"}
mkdir -p build
case "$os" in
 darwin)
  app="$root/build/Arduino Flasher UI Helper.app"
  mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
  $GO build -tags desktop,production -o "$app/Contents/MacOS/arduino-flasher-ui-helper" .
  cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>Arduino Flasher UI Helper</string>
<key>CFBundleDisplayName</key><string>Arduino Flasher UI Helper</string>
<key>CFBundleIdentifier</key><string>cc.jimbruges.arduino-flasher-ui-helper</string>
<key>CFBundleIconFile</key><string>AppIcon</string>
<key>CFBundleExecutable</key><string>arduino-flasher-ui-helper</string>
<key>CFBundleVersion</key><string>1</string>
<key>CFBundleShortVersionString</key><string>0.1.0</string>
<key>LSMinimumSystemVersion</key><string>12.0</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
PLIST
  cp packaging/icons/AppIcon.icns "$app/Contents/Resources/AppIcon.icns"
  if [ -d "$tools_dir" ]; then mkdir -p "$app/Contents/Resources/tools"; cp -R "$tools_dir"/. "$app/Contents/Resources/tools/"; fi
  # Bundle libusb for the digest helper, so the app works without Homebrew.
  packaged="$app/Contents/Resources/tools"
  if [ -x "$packaged/qdl-digest" ]; then
   usb=$(otool -L "$packaged/qdl-digest" | awk '/libusb.*dylib/{print $1;exit}')
   if [ -n "$usb" ] && [ -f "$usb" ]; then
    mkdir -p "$packaged/lib"
    if [ -f "$packaged/lib/libusb-1.0.0.dylib" ]; then chmod u+w "$packaged/lib/libusb-1.0.0.dylib"; fi
    cp "$usb" "$packaged/lib/libusb-1.0.0.dylib"
    install_name_tool -change "$usb" '@loader_path/lib/libusb-1.0.0.dylib' "$packaged/qdl-digest"
    codesign --force --sign - "$packaged/lib/libusb-1.0.0.dylib"
   fi
  fi
  for tool in arduino-flasher-cli qdl qdl-digest; do
   if [ -f "$packaged/$tool" ]; then codesign --force --sign - "$packaged/$tool"; fi
  done
  python3 scripts/check-mac-bundle.py "$app" --update-minimum
  codesign --force --deep --sign - "$app"
  printf 'Built %s\n' "$app"
 ;;
 linux)
  package="build/arduino-flasher-ui-helper-linux-$arch"
  mkdir -p "$package"
  $GO build -tags desktop,production,webkit2_41 -o "$package/arduino-flasher-ui-helper" .
  cp packaging/icons/app-icon.png "$package/app-icon.png"
  cp packaging/arduino-flasher-ui-helper.desktop "$package/arduino-flasher-ui-helper.desktop"
  if [ -d "$tools_dir" ]; then mkdir -p "$package/tools"; cp -R "$tools_dir"/. "$package/tools/"; fi
  printf 'Built %s\n' "$package"
 ;;
 windows)
  package="build/arduino-flasher-ui-helper-windows-$arch"
  mkdir -p "$package"
  CGO_ENABLED=0 $GO build -tags desktop,production -ldflags '-H windowsgui' -o "$package/arduino-flasher-ui-helper.exe" .
  if [ -d "$tools_dir" ]; then mkdir -p "$package/tools"; cp -R "$tools_dir"/. "$package/tools/"; fi
  cp packaging/WINDOWS.txt "$package/README.txt"
  printf 'Built %s\n' "$package"
 ;;
 *) echo 'Mac, Linux and Windows builds are supported.'  >&2; exit 1 ;;
esac
