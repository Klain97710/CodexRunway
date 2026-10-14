#!/usr/bin/env python3
"""Offer the renamed app as a manual install, never a cross-identity update."""
import json
import pathlib
import re
import sys
import xml.etree.ElementTree as ET

NS = "http://www.andymatuschak.org/xml-namespaces/sparkle"


def generate(config, tag):
    if config["identifier"] != "com.quosparr.app":
        raise ValueError("manual rename notice requires the Quosparr production identity")
    if not re.fullmatch(r"(?:beta-)?v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?", tag):
        raise ValueError("invalid release tag")
    ET.register_namespace("sparkle", NS)
    root = ET.Element("rss", version="2.0")
    channel = ET.SubElement(root, "channel")
    ET.SubElement(channel, "title").text = "Quosparr installation"
    item = ET.SubElement(channel, "item")
    ET.SubElement(item, "title").text = "Install Quosparr " + config["version"]
    ET.SubElement(item, f"{{{NS}}}version").text = str(config["macos"]["infoPlist"]["CFBundleVersion"])
    ET.SubElement(item, f"{{{NS}}}shortVersionString").text = config["version"]
    ET.SubElement(item, "link").text = "https://github.com/Licoy/Quosparr/releases/tag/" + tag
    ET.SubElement(item, "description").text = (
        "Quosparr has a new application identifier. Quit the previous app, install "
        "Quosparr, and launch it to migrate your local data."
    )
    return ET.ElementTree(root)


if __name__ == "__main__":
    config = json.loads(pathlib.Path(sys.argv[1]).read_text())
    generate(config, sys.argv[3]).write(sys.argv[2], encoding="utf-8", xml_declaration=True)
