# Arduino Flasher UI Helper

A native Mac/Linux wizard for installing factory images, capturing a configured
UNO Q or VENTUNO Q, and restoring a golden image to multiple boards. This is an
independent helper, not an official Arduino application.

## Quick start

1. Install Go 1.25.8 or newer, Git and native build dependencies:
   - **Mac:** install Xcode command-line tools (`xcode-select --install`), then
     `brew install go libusb libxml2 pkg-config`.
   - **Ubuntu 24.04:** install Go, then
     `sudo apt install build-essential git curl python3 pkg-config libusb-1.0-0-dev libxml2-dev libgtk-3-dev libwebkit2gtk-4.1-dev`.
2. From this repository, run:
   ```sh
   git submodule update --init --recursive
   scripts/build-tools.sh
   scripts/build.sh
   ```
3. Launch:
   - **Mac:** open `build/Arduino Flasher UI Helper.app`.
   - **Linux:** run `build/arduino-flasher-ui-helper-linux-amd64/arduino-flasher-ui-helper`
     (use `arm64` on ARM Linux).
4. Choose a workflow, select your board model, follow the illustrated EDL setup,
   refresh devices, select serials, and review the operation before starting.

No Node, npm, Wails CLI, browser server or GitHub login is required. The build
script downloads the submodule's pinned public QDL release using curl. For an
interface-only preview, skip building tools and launch with `--demo`:

```sh
# Mac
open 'build/Arduino Flasher UI Helper.app' --args --demo
# Linux
build/arduino-flasher-ui-helper-linux-amd64/arduino-flasher-ui-helper --demo
```

## Workflows

- **Factory image:** leave the image path empty to download the latest official
  image once, or choose a `.tar.zst`, `.tar.gz`, `.tgz`, `.tar`, `.zip` archive or
  extracted folder. Installation uses a shared prepared image with private GPT
  and XML files per target. VENTUNO Q passes every numbered rawprogram/patch XML
  pair to QDL, including additional LUNs. Factory success means QDL completed;
  it is not a complete sector verification or a promise that unused space is
  identical. Boot the board and confirm its image version afterward.
- **Capture golden image:** configure one source board with your apps, models
  and dependencies; shut Linux down cleanly, put it in EDL, choose its serial
  and a **new** output folder. This captures complete Linux storage. A separate
  MCU firmware image is not included. A local Firehose programmer is optional.
- **Restore golden image:** choose that folder and select matching target
  boards. The captured source is excluded automatically. Every source checksum
  is checked before writing, and every captured sector is verified afterward.
  Use matching storage capacities for identical layouts.

EDL discovery identifies serials, not models. Check the selected model yourself.
For VENTUNO Q, bridge JCTL pins 1 and 2, apply external power first, and then
connect USB data. Follow the official illustrations for each board. Remove the
bridge and power-cycle completed boards after a run.

## Fast batch settings

The default is **16 parallel boards**, 120 minutes per operation and
**Recommended** restoration. With the digest helper, this uses hierarchical
storage-digest comparison, 64 MiB comparison blocks, skips unchanged blocks, and
verifies the complete captured storage using device digests. Hash results cross
USB instead of reading the full image back into the computer.

Advanced settings also offer single-pass block digests, a full write with
verification, and USB readback comparison. Single-pass digests can help when
most blocks differ; a full write avoids pre-comparison when almost everything
must be replaced. Measure these against your actual factory image and USB
layout. The helper does not assume one strategy is fastest for every board.

Use powered hubs, reliable data cables and independent host USB controllers
where possible. More workers on one hub do not create more bandwidth. If the
digest helper is missing, Recommended falls back to USB readback, requiring
image-sized scratch files for active boards. Set `TMPDIR` before launching to
choose a disk with enough free space.

Earlier CLI measurements (not UI hardware tests): three full UNO Q restores
verified in **36m08s**; three mixed `--skip-unchanged` restores in **34m29s**.
The unchanged board finished in **17m52s**; the board needing 102 MiB finished
in **29m12s**, and the board needing 7.5 GiB finished in **34m23s**. These runs
used full USB readback, before digest acceleration. No 16-board benchmark has
been performed for this helper.

## Build and test

```sh
go test -race ./internal/...
go vet ./...
scripts/build.sh
```

The CI workflow builds natively on Mac and Ubuntu 24.04. Linux builds use
WebKitGTK 4.1. Mac bundles are ad-hoc signed for local use; public distribution
would require signing and notarisation. Use `BUILD_DIGEST=0 scripts/build-tools.sh`
only if you deliberately want to omit the optional digest helper.

The flasher branch is a Git submodule under `external/arduino-flasher-cli`, pinned
to a committed revision. Build scripts use a disposable `git archive` copy, so
the submodule remains untouched. The adapter supports both the legacy and
board-first golden command arguments. To use other tools, place
`arduino-flasher-cli`, `qdl` and optionally `qdl-digest` in a directory and pass
`--tools /absolute/path` or set `ARDUINO_FLASHER_UI_TOOLS_DIR`. A CLI without
`golden` still supports factory installation; golden features require the fork.

Each operation saves `events.jsonl`, `request.json` and `results.json` under the
local user configuration directory's `arduino-flasher-ui-helper/runs`. Open them
with the wizard's **Open saved run logs** button. Failed boards keep individual
results; cancellation stops CLI/QDL process groups. The app does not upload
images or logs. Demo mode uses simulated devices and never calls hardware tools.

## Attribution

Board/EDL illustrations are from Arduino's `arduino/docs-content` repository:
`content/software/app-lab/assets/playwright/flashing/uno-q-jctl-pins.png` and
`content/hardware/ventuno/boards/ventuno-q/tutorials/first-steps/flash-image/assets/jctl-edl-pins.png`.
Their upstream notices and terms apply; see `frontend/assets/ATTRIBUTION.md`.
Arduino Flasher CLI and its digest changes retain their upstream GPL notices.

Validation on 6 November 2026: Go race tests and vet passed; the Apple Silicon
native app, CLI and digest helper built successfully; bundle signature checks
passed. A native demo smoke test exercised discovery, selection, review, start
and completion for three simulated factory targets. Linux CI is configured but
has not been run from this local workspace. No hardware flashing was performed
while developing this helper.

To repeat the native bridge smoke test on Mac:

```sh
open -n 'build/Arduino Flasher UI Helper.app' --args --demo --smoke-report /tmp/arduino-ui-smoke.json
# After the simulated run finishes, the report should contain "success": true.
cat /tmp/arduino-ui-smoke.json
```
