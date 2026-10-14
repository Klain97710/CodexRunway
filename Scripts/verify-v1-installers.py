#!/usr/bin/env python3
"""Compare installer payloads with the authenticated update archives."""
import hashlib
import io
import json
import pathlib
import subprocess
import sys
import tarfile
import tempfile
import xml.etree.ElementTree as ET


def payloads(archive):
    with tarfile.open(archive,"r:gz") as tar:
        return {member.name.lstrip("./"):hashlib.sha256(tar.extractfile(member).read()).digest()
                for member in tar.getmembers() if member.isfile()}


def deb_members(file):
    data=file.read_bytes()
    if data[:8]!=b"!<arch>\n":
        raise ValueError("invalid Debian package")
    offset=8
    result={}
    while offset<len(data):
        header=data[offset:offset+60]
        if len(header)!=60 or header[58:]!=b"`\n":
            raise ValueError("invalid Debian member")
        size=int(header[48:58])
        result[header[:16].decode().strip().rstrip("/")]=data[offset+60:offset+60+size]
        offset+=60+size+(size%2)
    return result


def verify_deb(dist,version,arch):
    deb_version=version.replace("+","~")
    if "-" in deb_version:
        before,after=deb_version.split("-",1)
        deb_version=before+"~"+after.replace("-",".")
    members=deb_members(dist/f"quosparr_{deb_version}_{arch}.deb")
    with tarfile.open(fileobj=io.BytesIO(members["control.tar.gz"]),mode="r:gz") as tar:
        control_members={member.name.removeprefix("./"):member for member in tar.getmembers()}
        control=tar.extractfile(control_members["control"]).read().decode()
    for field in (f"Version: {deb_version}\n",f"Architecture: {arch}\n","libgtk-3-0","libayatana-appindicator3-1"):
        if field not in control:
            raise ValueError("Debian metadata or desktop dependencies mismatch")
    expected=payloads(dist/f"quosparr-{version}-linux-{arch}.tar.gz")
    with tarfile.open(fileobj=io.BytesIO(members["data.tar.gz"]),mode="r:gz") as tar:
        actual={member.name.removeprefix("./"):hashlib.sha256(tar.extractfile(member).read()).digest()
                for member in tar.getmembers() if member.isfile()}
    for name,digest in expected.items():
        if name.endswith((".desktop",".png",".xml")):
            continue
        if actual.get(f"opt/quosparr/{name}")!=digest:
            raise ValueError(f"Debian payload mismatch: {name}")


def verify_windows(dist,version,arch):
    expected=payloads(dist/f"quosparr-{version}-windows-{arch}.tar.gz")
    with tempfile.TemporaryDirectory(prefix="runway-nsis-") as temporary:
        setup=dist/f"Quosparr.Setup.{version}.{arch}.exe"
        subprocess.run(["7z","x","-tPE","-y",f"-o{temporary}/pe",str(setup),".rsrc/MANIFEST/1"],check=True,stdout=subprocess.DEVNULL)
        manifest=ET.parse(pathlib.Path(temporary)/"pe/.rsrc/MANIFEST/1")
        execution=manifest.findall(".//{*}requestedExecutionLevel")
        if len(execution)!=1 or execution[0].attrib.get("level")!="asInvoker":
            raise ValueError("Windows installer must request user privileges")
        subprocess.run(["7z","x","-y",f"-o{temporary}",str(setup)],check=True,stdout=subprocess.DEVNULL)
        if any(pathlib.Path(temporary).rglob("mygo-sqlite3.dll")):
            raise ValueError("Windows installer must not ship an external SQLite DLL")
        for name,digest in expected.items():
            matches=[path for path in pathlib.Path(temporary).rglob(pathlib.Path(name).name)
                     if path.is_file() and path.as_posix().endswith("/"+name)]
            if len(matches)!=1 or hashlib.sha256(matches[0].read_bytes()).digest()!=digest:
                raise ValueError(f"NSIS payload mismatch: {name}")


def main():
    dist,config=map(pathlib.Path,sys.argv[1:3])
    version=json.loads(config.read_text())["version"]
    for target in sys.argv[3].split(","):
        system,arch=target.split("-")
        if system=="linux": verify_deb(dist,version,arch)
        if system=="windows": verify_windows(dist,version,arch)
    print("Linux Debian and Windows installer payloads verified for selected targets")


if __name__=="__main__": main()
