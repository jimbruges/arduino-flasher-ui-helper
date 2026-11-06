#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
GO=${GO:-go}
os=$($GO env GOOS)
arch=$($GO env GOARCH)
case "$os" in
 darwin)
  ditto -c -k --sequesterRsrc --keepParent 'build/Arduino Flasher UI Helper.app' "build/arduino-flasher-ui-helper-macos-$arch.zip"
 ;;
 linux)
  tar -czf "build/arduino-flasher-ui-helper-linux-$arch.tar.gz" -C build "arduino-flasher-ui-helper-linux-$arch"
 ;;
 windows)
  powershell.exe -NoProfile -NonInteractive -Command "Compress-Archive -Path 'build/arduino-flasher-ui-helper-windows-$arch' -DestinationPath 'build/arduino-flasher-ui-helper-windows-$arch.zip' -Force"
 ;;
 *) echo 'Unsupported package platform.' >&2; exit 1 ;;
esac
