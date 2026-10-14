#!/usr/bin/env python3
import hashlib
import pathlib
import sys

mode,directory=sys.argv[1:3]
root=pathlib.Path(directory)
files=sorted(path for path in root.iterdir() if path.is_file() and
             (path.suffix in (".gz",".zip",".exe",".deb",".dmg",".xml",".json") or path.name.startswith("install-") or path.name=="install.sh"))
expected="".join(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n" for path in files)
if mode=="write":
    (root/"SHA256SUMS").write_text(expected)
elif mode=="verify":
    if (root/"SHA256SUMS").read_text()!=expected:
        raise SystemExit("release checksums do not match all selected assets")
else:
    raise SystemExit("usage: release-checksums.py write|verify DIST")
print(f"Release checksums {mode}: {len(files)} assets")
