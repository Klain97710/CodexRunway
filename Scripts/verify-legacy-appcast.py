#!/usr/bin/env python3
import json
import os
import pathlib
import subprocess
import sys
import xml.etree.ElementTree as ET

root=pathlib.Path(__file__).resolve().parent.parent
dist,config=map(pathlib.Path,sys.argv[1:3])
tag=sys.argv[3]
os.environ.setdefault("GOCACHE", str(root/".build/v1-go-cache"))
cfg=json.loads(config.read_text())
if tag != "personal-v"+cfg["version"]: raise SystemExit("Personal tag mismatch")
ns="http://www.andymatuschak.org/xml-namespaces/sparkle"
for arch in (sys.argv[4].split(",") if len(sys.argv)>4 else ("arm64","x86_64")):
    feed=ET.parse(dist/f"appcast-{arch}.xml")
    entries=feed.findall(".//item")
    if len(entries)!=1:
        raise SystemExit("legacy appcast must contain exactly the candidate item")
    enclosure=entries[0].find("enclosure")
    name=f"CodexRunway-macos-{arch}.zip"
    expected=f"https://github.com/Klain97710/CodexRunway/releases/download/{tag}/{name}"
    if enclosure.attrib["url"]!=expected:
        raise SystemExit("legacy archive URL mismatch")
    build=entries[0].findtext(f"{{{ns}}}version")
    if build is None: build=enclosure.attrib.get(f"{{{ns}}}version")
    if build!=str(cfg["macos"]["infoPlist"]["CFBundleVersion"]):
        raise SystemExit("legacy appcast build mismatch")
    version=entries[0].findtext(f"{{{ns}}}shortVersionString")
    if version is None: version=enclosure.attrib.get(f"{{{ns}}}shortVersionString")
    if version!=cfg["version"]:
        raise SystemExit("legacy appcast version mismatch")
    if int(enclosure.attrib["length"])!=(dist/name).stat().st_size:
        raise SystemExit("legacy archive length mismatch")
    signature=enclosure.attrib[f"{{{ns}}}edSignature"]
    subprocess.run(["go","run","./Scripts/release-assets","verify-sparkle","-config",str(config),"-file",str(dist/name),"-signature",signature],cwd=root,check=True)
    subprocess.run(["bash",str(root/"Scripts/sign-personal-appcast.sh"),"verify",str(dist/f"appcast-{arch}.xml")],check=True)
    # The official tool verifies the enclosure bytes as well as our Go verifier.
    tool=subprocess.check_output(["bash",str(root/"Scripts/prepare-personal-sparkle-tools.sh")],text=True).strip()
    subprocess.run([tool,"--verify","--ed-key-file","-",str(dist/name),signature],input=os.environ["SPARKLE_PRIVATE_KEY"],text=True,check=True)
print("Legacy XML feeds and ZIP signatures, versions and lengths verified")
