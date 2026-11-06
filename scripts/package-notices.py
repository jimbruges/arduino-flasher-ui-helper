#!/usr/bin/env python3
"""Retain original notices for Go modules actually linked into packaged binaries."""
import json, os, pathlib, re, shutil, subprocess, sys
root = pathlib.Path(__file__).resolve().parent.parent
dest = pathlib.Path(sys.argv[1])
dest.mkdir(parents=True, exist_ok=True)
# Upstream license files can be read-only; allow repeat builds to replace them.
for existing in dest.rglob('*'):
    if existing.is_file():
        existing.chmod(existing.stat().st_mode | 0o200)
for name in ('LICENSE', 'NOTICE.md'):
    shutil.copy2(root / name, dest / name)
shutil.copytree(root / 'LICENSES', dest / 'LICENSES', dirs_exist_ok=True)
def revision(directory):
    return subprocess.check_output(['git', '-C', str(directory), 'rev-parse', 'HEAD'], text=True).strip()
helper_revision = revision(root)
cli_revision = revision(root / 'external/arduino-flasher-cli')
source_links = [
    '# Source for this build',
    '',
    'Helper: https://github.com/jimbruges/arduino-flasher-ui-helper/tree/' + helper_revision,
    'CLI and digest patch/build scripts: https://github.com/jimbruges/arduino-flasher-cli/tree/' + cli_revision,
    'QDL: https://github.com/linux-msm/qdl/tree/v2.4',
    'QDL packaging patches and build recipe: https://github.com/arduino/qdl-packing/tree/v2.4-26',
    'QDL dependency build recipes: https://github.com/arduino/crossbuild/tree/0.3.0-pre.1',
    'Mac ARM64 QDL dependency recipe (upstream uses a moving branch): https://github.com/arduino/crossbuild/blob/main/scripts/build_mac.sh',
    'Local macOS build libusb 1.0.30 source: https://github.com/libusb/libusb/releases/tag/v1.0.30',
    '',
    'Build instructions: see README.md and scripts/ in the helper source above.',
    'Clone with --recurse-submodules to obtain the pinned CLI source.',
    'Go module versions and source download links are listed below.',
    'These links identify available sources; the prebuilt QDL recipe does not pin every dependency on every platform.',
    '',
]
go = os.environ.get('GO', 'go')
modules = set()
for binary in sys.argv[2:]:
    if not pathlib.Path(binary).is_file():
        if binary == sys.argv[2]:
            raise SystemExit('Missing helper executable: ' + binary)
        continue  # Interface-only demo builds have no bundled CLI.
    text = subprocess.check_output([go, 'version', '-m', binary], text=True)
    modules.update(re.findall(r'^\s*dep\s+(\S+)\s+(\S+)', text, re.M))
cache = pathlib.Path(subprocess.check_output([go, 'env', 'GOMODCACHE'], text=True).strip())
sections = ['Third-party Go module licenses\nVersions extracted from the packaged executables.\n']
for module, version in sorted(modules):
    source_links.append(module + '@' + version + ': https://proxy.golang.org/' + ''.join('!' + c.lower() if c.isupper() else c for c in module) + '/@v/' + version + '.zip')
    escaped = ''.join('!' + c.lower() if c.isupper() else c for c in module)
    directory = cache / (escaped + '@' + version)
    if not directory.is_dir():
        info = json.loads(subprocess.check_output([go, 'mod', 'download', '-json', module + '@' + version], text=True))
        directory = pathlib.Path(info['Dir'])
    files = sorted(p for p in directory.rglob('*') if p.is_file() and re.match(r'^(license|copying|copyright|notice)(\b|[._-])', p.name, re.I))
    if not files:
        raise SystemExit('No upstream license found: ' + module + '@' + version)
    sections.append('\n' + '=' * 72 + '\n' + module + '@' + version + '\n')
    for path in files:
        sections.append('\n--- ' + str(path.relative_to(directory)) + ' ---\n' + path.read_text(errors='replace') + '\n')
goroot = pathlib.Path(subprocess.check_output([go, 'env', 'GOROOT'], text=True).strip())
sections.append('\nGo toolchain/runtime license\n' + (goroot / 'LICENSE').read_text())
source_links.append('Go toolchain: https://go.dev/dl/#' + subprocess.check_output([go, 'env', 'GOVERSION'], text=True).strip())
(dest / 'SOURCE.md').write_text('\n'.join(source_links) + '\n')
(dest / 'GO-MODULE-NOTICES.txt').write_text(''.join(sections))
print('Included notices for ' + str(len(modules)) + ' linked Go modules.')
