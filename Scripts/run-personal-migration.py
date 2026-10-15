#!/usr/bin/env python3
"""Run a fixture-only developer package in a disposable macOS test session.

Never changes HOME, UserDefaults, Keychain, installed apps or other processes.
Create a new private empty directory with --init; review it before --app.
"""
import argparse
import json
import os
from pathlib import Path
import plistlib
import pwd
import subprocess

MARKER = '.codex-runway-migration-sandbox'
MARKER_VALUE = 'synthetic-fixtures-only-v1\n'


def validate(root):
    root = Path(os.path.abspath(root))
    real_home = Path(pwd.getpwuid(os.getuid()).pw_dir).resolve()
    if root.resolve() != root or root == real_home or root in real_home.parents:
        raise ValueError('sandbox may not alias the user home or contain it')
    for name in ('.codex', '.codex-runway', '.quosparr', '.grok', '.claude', 'Library'):
        protected = real_home / name
        if root == protected or protected in root.parents or root in protected.parents:
            raise ValueError('sandbox overlaps production data')
    if not root.is_dir() or root.stat().st_mode & 0o077:
        raise ValueError('sandbox must be a private directory (0700)')
    if any(p.is_symlink() for p in root.rglob('*')):
        raise ValueError('symlinks are forbidden in fixture sandboxes')
    if (root / MARKER).read_text() != MARKER_VALUE:
        raise ValueError('sandbox marker missing or invalid')
    return root


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sandbox', type=Path, required=True)
    parser.add_argument('--init', action='store_true', help='create a new empty fixture directory; never imports user data')
    parser.add_argument('--app', type=Path, help='packaged CodexRunway Migration.app')
    parser.add_argument('--check', action='store_true', help='validate only; do not launch')
    args, app_args = parser.parse_known_args()
    if args.init:
        # exist_ok=False deliberately refuses to mark an existing data tree.
        args.sandbox.mkdir(mode=0o700, parents=False, exist_ok=False)
        (args.sandbox / MARKER).write_text(MARKER_VALUE)
        (args.sandbox / MARKER).chmod(0o600)
    root = validate(args.sandbox)
    if args.check or args.init and args.app is None:
        print('Validated private synthetic-fixture sandbox.')
        return
    if args.app is None:
        parser.error('--app is required to run; first build RUNWAY_BUILD_PROFILE=migration-dev')
    app = args.app.resolve()
    info = plistlib.loads((app / 'Contents/Info.plist').read_bytes())
    if info.get('CFBundleIdentifier') != 'com.github.codex-runway.migration' or info.get('RunwayBuildProfile') != 'migration-dev':
        raise ValueError('only the isolated migration-dev package may be launched here')
    if not info['CFBundleExecutable'] or Path(info['CFBundleExecutable']).name != info['CFBundleExecutable']:
        raise ValueError('invalid executable')
    subprocess.run(['codesign', '--verify', '--deep', '--strict', str(app)], check=True)
    env = os.environ.copy()
    for key in list(env):
        if key.startswith(('QUOSPARR_', 'RUNWAY_', 'SPARKLE_', 'MYGO_')) or key in ('CODEX_HOME', 'GROK_HOME', 'CLAUDE_CONFIG_DIR'):
            env.pop(key)
    env.update(RUNWAY_MIGRATION_SANDBOX=str(root), CODEX_HOME=str(root / '.codex'), GROK_HOME=str(root / '.grok'), CLAUDE_CONFIG_DIR=str(root / '.claude'))
    child = subprocess.Popen([str(app / 'Contents/MacOS' / info['CFBundleExecutable']), *app_args], env=env)
    try:
        raise SystemExit(child.wait())
    finally:
        if child.poll() is None:
            child.terminate()
            try:
                child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()


if __name__ == '__main__':
    main()
