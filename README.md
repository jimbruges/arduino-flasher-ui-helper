# Arduino Flasher UI Helper

A native Mac/Linux/Windows wizard for installing factory images, capturing a configured
UNO Q or VENTUNO Q, and restoring a golden image to multiple boards. This is an
independent helper, not an official Arduino application.

## Quick start

1. Install Go 1.25.8 or newer, Git and native build dependencies:
   - **Mac:** install Xcode command-line tools (`xcode-select --install`), then
     `brew install go libusb libxml2 pkg-config`.
   - **Windows x64:** install Go, Python 3 and Git for Windows, then run these build
     commands in Git Bash. WebView2 and a QDL-compatible USB driver are needed
     to run the application. The Windows build uses readback verification.
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
   - **Windows:** open `build/arduino-flasher-ui-helper-windows-amd64/arduino-flasher-ui-helper.exe`.
4. Choose a task on the start screen, then follow **Board → Image → Connect → Review**.
   Select the large board card, choose the image, follow the illustrated EDL
   setup and select serials. Review the automatically selected strategy before starting.

No Node, npm, Wails CLI, browser server or GitHub login is required to build. The build
script downloads the submodule's pinned public QDL release using curl. For an
interface-only preview, skip building tools and launch with `--demo`:

```sh
# Mac
open 'build/Arduino Flasher UI Helper.app' --args --demo
# Linux
build/arduino-flasher-ui-helper-linux-amd64/arduino-flasher-ui-helper --demo
```

## Running packaged builds

- **Mac:** open the `.app`. It bundles the CLI, QDL, digest helper and libusb;
  Homebrew and build tools are not needed to run it. The current local build is
  **Apple Silicon, macOS 26 or newer**. The build checks bundled dependencies
  and sets the minimum macOS version from the actual binaries. Downloaded copies
  are ad-hoc signed rather than notarised and may require approval to open.
- **Windows x64:** extract the whole ZIP and open the `.exe`, keeping `tools`
  alongside it. Install Microsoft Edge WebView2 Runtime if missing and a
  QDL-compatible USB driver for EDL devices. This preview includes CLI/QDL but
  uses complete USB readback verification; the digest helper is not packaged.
  It has been cross-compiled, not run or hardware-tested on Windows.
- **Linux:** build natively using Quick start, or use the Ubuntu CI artifact once binary distribution is enabled.
  Runtime packages on Ubuntu 24.04 are `libgtk-3-0t64`, `libwebkit2gtk-4.1-0`,
  `libusb-1.0-0` and `libxml2`. USB permissions must allow access to EDL devices.
  Linux builds are configured in CI but have not yet been validated here.

The simple app icon is included in the Mac bundle, Windows executable and Linux
window/package. Its generated assets are in `packaging/icons`; the Mac drawing
source is `scripts/make-icon.swift`. Windows icon/manifest resources are stored
in `app_windows_amd64.syso` so normal builds need no resource compiler.

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
  After capture, choose **Source unplugged — connect targets** to restore the
  same image to new boards. Unplug the source, put the targets in EDL mode and
  connect them. The wizard keeps the image and model, excludes the source and
  selects all discovered targets; review the selection before writing.
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
**Automatic** restoration. The reviewed choice is applied to the actual command;
it uses your board model, image workflow, expected target contents and number
of active workers **after excluding the captured source**.

| Image / targets | Automatic choice |
| --- | --- |
| Official factory image | Prepare once, flash the declared partitions on each board |
| Golden image, one active VENTUNO Q with factory/unknown contents | Full write + digest verification |
| Golden image, multiple active boards with factory/unknown contents | Single-pass 64 MiB block comparison, changed writes + digest verification |
| Golden image, one UNO Q, or targets already close to the image | Hierarchical comparison, changed writes + digest verification |
| Digest helper unavailable, factory/unknown target contents | Full write + complete USB readback verification |
| Digest helper unavailable, targets close to the image | USB readback comparison, changed writes + complete readback verification |

Choose **Already close to this golden image** when updating previously restored
boards. Otherwise leave **Factory image or unknown contents** selected. In
**Advanced settings**, you can override Automatic with hierarchical digests,
single-pass block digests, full write or readback. Unsupported manual digest
choices are rejected rather than silently substituted. If the CLI lacks block
comparison, Automatic uses hierarchical digests for UNO Q and full write with
digest verification for VENTUNO Q.

These defaults are benchmark-informed recommendations, not a guarantee of the
fastest result on every USB topology. Recorded single-board VENTUNO Q full
write + digest verification took **36m47s**, versus **45m46s** for the earlier
factory partial restore. Multiple active boards favor comparison to reduce
shared USB write traffic; a factory-content single-pass batch and 16-board
throughput have not been benchmarked. All golden strategies retain complete
captured-storage verification. Factory installation retains QDL completion
semantics, not full-sector verification.

Use powered hubs, reliable data cables and independent host USB controllers
where possible. More workers on one hub do not create more bandwidth. If the
digest helper is missing, Automatic falls back to USB readback, requiring
image-sized scratch files for active boards. Set `TMPDIR` before launching to
choose a disk with enough free space.

Earlier CLI measurements (not UI hardware tests): three full UNO Q restores
verified in **36m08s**; three mixed `--skip-unchanged` restores in **34m29s**.
The unchanged board finished in **17m52s**; the board needing 102 MiB finished
in **29m12s**, and the board needing 7.5 GiB finished in **34m23s**. These runs
used full USB readback, before digest acceleration. No 16-board benchmark has
been performed for this helper.

## Transfer sizes and rough timing

Review shows the maximum data written per target and across the batch, together
with benchmark references. Typical complete captures are **14.60 GiB for UNO Q**
and **59.31 GiB for VENTUNO Q**. At 16 targets that is up to **233.58 GiB** or
**948.99 GiB** of writes respectively, plus verification. Changed-block strategies
reduce writes after comparison; the review cannot know the changed amount yet.
Live transfer cards show completed/total data when available; final results show
written and skipped bytes. Digests avoid transferring a complete verification
readback over USB.

| Measured CLI run | Size | Time |
| --- | --- | --- |
| UNO Q capture, one source | 14.60 GiB | 2m44s |
| UNO Q full restore + USB readback, one target | 14.60 GiB | 16m31s |
| UNO Q two matching targets, digest comparison | 14.60 GiB each; zero writes | 2m23s batch |
| UNO Q two partial digest restores | 10.85 GiB written each | 16m09s batch |
| VENTUNO Q capture over USB 2 | 59.31 GiB | 28m53s |
| VENTUNO Q full restore + digests, one target | 59.31 GiB | 36m47s |
| VENTUNO Q factory partial golden restore | 50.59 GiB written | 45m46s |
| VENTUNO Q matching block comparison | 59.31 GiB checked; zero writes | 7m18s |

Selecting a golden folder displays its exact image size from the manifest.
Review uses that size for per-board and batch data totals, with a short,
size-scaled total timing range for one target. Multi-board runs do not reuse
that range: concurrent workers share USB bandwidth, and there is no reliable
batch prediction yet. The run shows actual batch elapsed time and each board’s
current step ETA. Matching boards can finish sooner. Each board card shows completed/total data for its
current step, elapsed time and step ETA. The detailed benchmark references stay
here rather than filling the wizard. Estimates are rough guides, not measured
predictions. Neither 16-board throughput nor factory archive
installation has been benchmarked. Factory archive size is compressed; the
partitions declared by the extracted image determine the transfer, rather than
the full storage capacity. After preparation, the run displays the total declared
programming ranges per board and batch when the XML provides numeric sizes;
sparse images and GPT adjustments can change the actual USB bytes.

## Build and test

```sh
go test -race ./internal/...
go vet ./...
scripts/build.sh
```

The CI workflow builds on Mac, Ubuntu 24.04 and Windows and prepares
Mac/Windows ZIPs and a Linux tarball. Public artifact uploads are disabled until
corresponding GPL/LGPL sources and applicable relinking materials are supplied;
then set repository variable `PUBLISH_BINARY_ARTIFACTS=true`.
The archives preserve executable permissions.
To package a local build, run `scripts/package.sh`. Linux builds use
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

## Licensing

The helper is GPL v3; dependencies and illustrations retain their own licenses.
See `NOTICE.md`, `LICENSES` and `frontend/assets/ATTRIBUTION.md`. Builds include
original license notices for linked Go modules and bundled flashing tools.
Source-only publication and binary-release compliance are separate: binary
releases still need corresponding GPL/LGPL source and applicable relinking
materials alongside the executable downloads.

## Attribution

Board/EDL illustrations are from Arduino's `arduino/docs-content` repository:
`content/software/app-lab/assets/playwright/flashing/uno-q-jctl-pins.png` and
`content/hardware/ventuno/boards/ventuno-q/tutorials/first-steps/flash-image/assets/jctl-edl-pins.png`.
Their upstream notices and terms apply; see `frontend/assets/ATTRIBUTION.md`.
Arduino Flasher CLI and its digest changes retain their upstream GPL notices.

Validation on 6 November 2026: Go race tests and vet passed; the Apple Silicon
native app, CLI and digest helper built successfully; bundle signature checks
passed. A native demo smoke test exercised discovery, selection, review, start
and completion for three simulated factory targets. The redesigned wizard also
checks explicit board selection, large selector dimensions, local-image selection,
source selection, hardware confirmation and write confirmation. Automatic strategy
tests cover single/parallel batches, source exclusion, missing helpers and manual overrides. Linux CI is configured but
has not been run from this local workspace. No hardware flashing was performed
while developing this helper.

To repeat the native bridge smoke test on Mac:

```sh
open -n 'build/Arduino Flasher UI Helper.app' --args --demo --smoke-report /tmp/arduino-ui-smoke.json
# After the simulated run finishes, the report should contain "success": true.
cat /tmp/arduino-ui-smoke.json
```
