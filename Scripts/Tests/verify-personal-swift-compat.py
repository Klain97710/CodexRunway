#!/usr/bin/env python3
"""Compile only the frozen Swift Core; never build/run its desktop entry point."""
from pathlib import Path
import os
import shutil
import subprocess
import tempfile

ROOT=Path(__file__).resolve().parents[2]
BASE='895e991d67eea3f8500065b605741f42aee54ae3'
work=ROOT/'.build/personal-swift-compat'
work.mkdir(parents=True,exist_ok=True)
archive=subprocess.run(['git','archive',BASE,'Sources/CodexRunwayCore'],cwd=ROOT,check=True,stdout=subprocess.PIPE).stdout
subprocess.run(['tar','-xf','-','-C',str(work)],input=archive,check=True)
(work/'Package.swift').write_text('''// swift-tools-version: 6.0
import PackageDescription
let package = Package(name: "MigrationCompatibility", platforms: [.macOS(.v12)], targets: [
.target(name: "CodexRunwayCore", linkerSettings: [.linkedLibrary("sqlite3")]),
.executableTarget(name: "CompatibilityProbe", dependencies: ["CodexRunwayCore"])])
''')
probe=work/'Sources/CompatibilityProbe'
probe.mkdir(parents=True,exist_ok=True)
shutil.copyfile(ROOT/'Scripts/Tests/personal-swift-compat.swift',probe/'Probe.swift')
subprocess.run(['swift','build','--package-path',str(work),'--product','CompatibilityProbe'],check=True)
with tempfile.TemporaryDirectory(prefix='personal-synthetic-compat-') as temp:
    data=Path(temp)/'home/.codex'
    for folder in ('sessions','archived_sessions'):
        shutil.copytree(ROOT/'internal/usage/testdata/swift_compat'/folder,data/folder)
    binary=work/'.build/debug/CompatibilityProbe'
    subprocess.run([str(binary),'emit',temp,str(ROOT)],check=True)
    subprocess.run(['go','run','./Scripts/personal-compat',temp],cwd=ROOT,check=True)
    subprocess.run(['go','test','./internal/usage','-count=1','-run','^TestCrossImplementationIndexReuse$'],cwd=ROOT,env=dict(os.environ,USAGE_COMPAT_DIR=temp,USAGE_REQUIRE_SQLITE='1'),check=True)
    subprocess.run([str(binary),'verify',temp],check=True)
