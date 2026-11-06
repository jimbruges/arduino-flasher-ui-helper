#!/usr/bin/env python3
"""Retain original notices for Go modules actually linked into packaged binaries."""
import json, os, pathlib, re, shutil, subprocess, sys
root = pathlib.Path(__file__).resolve().parent.parent
dest = pathlib.Path(sys.argv[1])
dest.mkdir(parents=True, exist_ok=True)
for name in ('LICENSE', 'NOTICE.md'):
    shutil.copy2(root / name, dest / name)
shutil.copytree(root / 'LICENSES', dest / 'LICENSES', dirs_exist_ok=True)
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
(dest / 'GO-MODULE-NOTICES.txt').write_text(''.join(sections))
print('Included notices for ' + str(len(modules)) + ' linked Go modules.')
