#!/usr/bin/env python3
"""Keep mygo's packaging, omitting only the SQLite DLL embedded by Quosparr."""
import hashlib
import json
import pathlib
import shutil
import sys


def prepare(module, destination):
    root = pathlib.Path(__file__).resolve().parents[1]
    manifest = json.loads((module / "plugins/sqlite/mygo-plugin.json").read_text())
    libraries = [entry for entry in manifest["libraries"] if entry["name"] == "mygo-sqlite3"]
    if len(libraries) != 1:
        raise ValueError("expected one pinned SQLite library")
    for arch in ("amd64", "arm64"):
        data = (root / f"internal/sqliteruntime/assets/{arch}/mygo-sqlite3.dll").read_bytes()
        if hashlib.sha256(data).hexdigest() != libraries[0]["files"][f"windows-{arch}"]["sha256"]:
            raise ValueError(f"embedded Windows {arch} SQLite differs from pinned mygo runtime")

    source = module / "cmd/mygo/natives.go"
    text = source.read_text()
    anchor = "\t\tfor _, l := range n.Libraries {\n"
    if text.count(anchor) != 1:
        raise ValueError("unsupported mygo native-resource discovery; review the pinned CLI")
    replacement = anchor + ('\t\t\tif goos == "windows" && l.Name == "mygo-sqlite3" {\n'
                            '\t\t\t\tcontinue // embedded by Quosparr\n\t\t\t}\n')
    # Go forbids overlays beneath GOMODCACHE. Keep the pinned CLI's module
    # identity and internal imports in a disposable, writable build copy.
    cli = destination / "mygo-cli"
    shutil.copytree(module, cli, copy_function=shutil.copyfile)
    # copytree preserves the module cache's read-only directory modes even
    # when copyfile makes writable files; this build copy must be removable.
    for path in [cli, *(path for path in cli.rglob("*") if path.is_dir())]:
        path.chmod(0o755)
    (cli / "cmd/mygo/natives.go").write_text(text.replace(anchor, replacement))


if __name__ == "__main__":
    prepare(*map(pathlib.Path, sys.argv[1:3]))
