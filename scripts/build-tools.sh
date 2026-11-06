#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO=${GO:-go}
source_dir="$root/external/arduino-flasher-cli"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM
tools_dir=${TOOLS_DIR:-"$root/tools"}
mkdir -p "$work/source" "$tools_dir"
# Generated files belong to this disposable copy, never to the submodule.
git -C "$source_dir" archive HEAD | tar -xf - -C "$work/source"
os=$($GO env GOOS)
arch=$($GO env GOARCH)
case "$os/$arch" in
 darwin/arm64) asset=macOS_arm64 ;;
 darwin/amd64) asset=macOS_64bit ;;
 linux/amd64) asset=Linux_64bit ;;
 linux/arm64) asset=Linux_ARM64 ;;
 windows/amd64) asset=Windows_32bit ;;
 *) echo 'Build tools on a supported Mac, Linux or Windows target.' >&2; exit 1 ;;
esac
tool_suffix=
if [ "$os" = windows ]; then tool_suffix=.exe; fi
digest_default=1
if [ "$os" = windows ]; then digest_default=0; fi
tag=$(sed -n 's/^TAG="\([^"]*\)"/\1/p' "$work/source/internal/updater/artifacts/download_resources.sh")
test -n "$tag"
resources="$work/source/internal/updater/artifacts/resources_${os}_${arch}"
mkdir -p "$resources"
curl --fail --silent --show-error --location "https://github.com/arduino/qdl-packing/releases/download/$tag/qdl_${tag}_${asset}.tar.gz" -o "$work/qdl.tar.gz"
tar -xzf "$work/qdl.tar.gz" -C "$resources" "qdl$tool_suffix"
cp "$resources/qdl$tool_suffix" "$tools_dir/qdl$tool_suffix"
cd "$work/source"
$GO build -o "$tools_dir/arduino-flasher-cli$tool_suffix" ./cmd/arduino-flasher-cli
if [ "${BUILD_DIGEST:-$digest_default}" = 1 ]; then
 if [ "$os" = windows ]; then
  echo 'Windows digest helper is not packaged yet; use BUILD_DIGEST=0 (readback verification).' >&2; exit 1
 fi
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
 cp build/qdl-digest "$tools_dir/qdl-digest"
fi
chmod +x "$tools_dir/arduino-flasher-cli$tool_suffix" "$tools_dir/qdl$tool_suffix"
printf '%s\n' 'Tools built. Submodule remains unchanged.'
