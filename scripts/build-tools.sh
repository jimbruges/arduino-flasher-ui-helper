#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO=${GO:-go}
source_dir="$root/external/arduino-flasher-cli"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM
mkdir -p "$work/source" "$root/tools"
# Generated files belong to this disposable copy, never to the submodule.
git -C "$source_dir" archive HEAD | tar -xf - -C "$work/source"
os=$($GO env GOOS)
arch=$($GO env GOARCH)
case "$os/$arch" in
 darwin/arm64) asset=macOS_arm64 ;;
 darwin/amd64) asset=macOS_64bit ;;
 linux/amd64) asset=Linux_64bit ;;
 linux/arm64) asset=Linux_ARM64 ;;
 *) echo 'Build tools on a supported Mac or Linux host.' >&2; exit 1 ;;
esac
tag=$(sed -n 's/^TAG="\([^"]*\)"/\1/p' "$work/source/internal/updater/artifacts/download_resources.sh")
test -n "$tag"
resources="$work/source/internal/updater/artifacts/resources_${os}_${arch}"
mkdir -p "$resources"
curl --fail --silent --show-error --location "https://github.com/arduino/qdl-packing/releases/download/$tag/qdl_${tag}_${asset}.tar.gz" -o "$work/qdl.tar.gz"
tar -xzf "$work/qdl.tar.gz" -C "$resources" qdl
cp "$resources/qdl" "$root/tools/qdl"
cd "$work/source"
$GO build -o "$root/tools/arduino-flasher-cli" ./cmd/arduino-flasher-cli
if [ "${BUILD_DIGEST:-1}" = 1 ]; then
 # On Mac, use the SDK's libxml2 when Homebrew only provides libusb.
 if [ "$os" = darwin ] && ! pkg-config --exists libxml-2.0; then
  sdk=$(xcrun --show-sdk-path)
  mkdir -p "$work/pkgconfig"
  cat > "$work/pkgconfig/libxml-2.0.pc" <<PC
Name: libxml2
Description: macOS SDK XML library
Version: 2.9.0
Cflags: -I$sdk/usr/include/libxml2
Libs: -lxml2
PC
  export PKG_CONFIG_PATH="$work/pkgconfig:${PKG_CONFIG_PATH:-}"
 fi
 ./scripts/qdl-digest/build.sh
 cp build/qdl-digest "$root/tools/qdl-digest"
fi
chmod +x "$root/tools/arduino-flasher-cli" "$root/tools/qdl"
printf '%s\n' 'Tools built. Submodule remains unchanged.'
