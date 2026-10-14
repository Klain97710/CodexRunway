#!/usr/bin/env python3
"""Validate actual Mach-O deployment targets, not just Info.plist claims."""
from pathlib import Path
import json
import plistlib
import re
import subprocess
import sys


def version(value):
    parts=tuple(map(int,value.split('.')))
    return parts+(0,)*(3-len(parts))


def check_minimum(output, declared):
    targets=re.findall(r'^\s*minos\s+([0-9.]+)\s*$',output,re.M)
    if not targets: raise ValueError('Mach-O LC_BUILD_VERSION minimum is missing')
    if any(version(target)>version(declared) for target in targets):
        raise ValueError(f'Mach-O minimum {max(targets,key=version)} exceeds declared macOS {declared}')


def main():
    app=Path(sys.argv[1]); config=json.loads(Path(sys.argv[2]).read_text())
    contents=app/'Contents'
    info=plistlib.loads((contents/'Info.plist').read_bytes())
    widget=contents/'PlugIns/QuosparrWidget.appex/Contents'
    widget_info=plistlib.loads((widget/'Info.plist').read_bytes())
    checks=[(contents/'MacOS'/info['CFBundleExecutable'],info['LSMinimumSystemVersion']),
            (contents/'Frameworks/libQuosparrWidgetBridge.dylib',info['LSMinimumSystemVersion']),
            (contents/'Resources/libmygo-sqlite3.dylib',info['LSMinimumSystemVersion']),
            (widget/'MacOS/QuosparrWidget',widget_info['LSMinimumSystemVersion'])]
    for binary,minimum in checks:
        output=subprocess.check_output(['xcrun','vtool','-show-build',str(binary)],text=True)
        try: check_minimum(output,minimum)
        except ValueError as e: raise SystemExit(f'{binary.name}: {e}')
        linked=subprocess.check_output(['otool','-L',str(binary)],text=True)
        for library in re.findall(r'^\s+(\S+)\s+\(compatibility version',linked,re.M):
            if library.startswith('/') and not library.startswith(('/System/Library/','/usr/lib/')):
                raise SystemExit(f'{binary.name}: nonportable library dependency')
    data=checks[0][0].read_bytes()
    if info['RunwayBuildProfile']=='replacement-candidate':
        updates=config['updates']
        for marker in (updates['publicKey'],'https://github.com/'+updates['github']+'/releases/download/'+updates['tagPrefix']+'{version}/'):
            if marker.encode() not in data: raise SystemExit('Bundled mygo update key/feed mismatch')
    print('Mach-O minimum OS, library paths and embedded update configuration verified')

if __name__=='__main__': main()
