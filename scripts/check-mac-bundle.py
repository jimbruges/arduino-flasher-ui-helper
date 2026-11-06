#!/usr/bin/env python3
"""Check that Mach-O dependencies are bundled or supplied by macOS."""
import pathlib, plistlib, re, subprocess, sys
app = pathlib.Path(sys.argv[1]).resolve()
minimum = (12, 0)
checked = 0
for path in (app / 'Contents').rglob('*'):
    if not path.is_file():
        continue
    if 'Mach-O' not in subprocess.check_output(['file', '-b', str(path)], text=True):
        continue
    checked += 1
    build = subprocess.check_output(['vtool', '-show-build', str(path)], text=True)
    for version in re.findall(r'^\s*minos\s+(\d+(?:\.\d+)+)', build, re.M):
        minimum = max(minimum, tuple(map(int, version.split('.'))))
    own_ids = subprocess.check_output(['otool', '-D', str(path)], text=True).splitlines()[1:]
    links = subprocess.check_output(['otool', '-L', str(path)], text=True).splitlines()[1:]
    for line in links:
        dependency = line.strip().split(' (', 1)[0]
        if dependency in own_ids or dependency.startswith(('/usr/lib/', '/System/Library/')):
            continue
        if dependency.startswith('@loader_path/'):
            target = path.parent / dependency.removeprefix('@loader_path/')
            if target.is_file() and target.resolve().is_relative_to(app):
                continue
        raise SystemExit(f'External dependency: {path.name}: {dependency}')
version = '.'.join(map(str, minimum))
if '--update-minimum' in sys.argv:
    path = app / 'Contents/Info.plist'
    info = plistlib.loads(path.read_bytes())
    info['LSMinimumSystemVersion'] = version
    path.write_bytes(plistlib.dumps(info))
print(f'Checked {checked} binaries: bundled/system dependencies only; macOS {version}+ required.')
