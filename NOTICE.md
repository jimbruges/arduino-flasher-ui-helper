# Licensing and source

The helper's original code, UI and simple app icon are distributed under GNU
GPL version 3 (`LICENSE`). Third-party components retain their original licenses
and copyright notices; the helper license does not replace those terms.

- **Arduino Flasher CLI:** GPL-3.0-or-later. Exact source is the commit pinned in
  `external/arduino-flasher-cli`; `.gitmodules` points to Jim Bruges's public fork.
- **QDL:** upstream BSD-3-Clause, copyright Linaro Ltd. and Bjorn Andersson.
  Its sparse-image header is Apache-2.0, copyright The Android Open Source
  Project; that notice and license are also retained.
  The prebuilt binary comes from Arduino's qdl-packing v2.4-26 release. The digest
  helper applies this fork's storage-digest patch to QDL v2.4; the added code is
  covered by the CLI fork's GPL license. Both the original notices and patched
  source/build instructions must accompany binary distribution.
- **Go modules:** Wails is MIT and klauspost/compress is BSD-3-Clause. Other
  modules retain their own licenses. Package generation collects the original
  notices for the exact module versions linked into the packaged executables.
- **libusb:** LGPL-2.1-or-later. The local Mac package uses libusb 1.0.30 from
  Homebrew and dynamically links the bundled library. Other targets may link it
  statically inside QDL. Do not treat a license-text bundle alone as satisfying
  corresponding-source or relinking requirements for binary releases.
- **libxml2:** retains its upstream copyright/license. macOS's system copy is
  not redistributed; prebuilt QDL may include a static copy.
- **Windows driver files:** the CLI embeds qcserlib.inf/qcserlib.cat. Its pinned
  upstream `NOTICE.md` identifies these as Qualcomm's BSD-3-Clause files. Those
  notices are retained; this helper does not independently relicense them.
- **Arduino illustrations:** retained under the docs-content repository's full
  CC-BY-SA-4.0 license text, separately from the application code. See
  `frontend/assets/ATTRIBUTION.md` and `LICENSES/Arduino-docs-content.md`.

License texts are in `LICENSES`. Binary packages include these notices and
licenses under `licenses/` (inside Contents/Resources on Mac). `SOURCE.md` in
that folder links the helper build commit, pinned CLI/digest sources, QDL v2.4,
Arduino packaging patches/build recipes and versioned Go module source downloads.

Helper source: https://github.com/jimbruges/arduino-flasher-ui-helper
CLI fork: https://github.com/jimbruges/arduino-flasher-cli
QDL build recipe: https://github.com/arduino/qdl-packing/tree/v2.4-26

The prebuilt Mac ARM64 QDL recipe references crossbuild's moving `main` branch;
its exact historical dependency inputs remain unverified. Source links and
license notices do not themselves resolve any additional LGPL requirements
for statically linked libraries.
