#!/usr/bin/env python3
"""Stage the existing mygo config without embedding signing credentials."""
import json
import os
import pathlib
import re
import sys

root, destination = map(pathlib.Path, sys.argv[1:3])
source = pathlib.Path(os.environ.get("V1_CONFIG_PATH", root / "mygo.json"))
config = json.loads(source.read_text())
if not re.fullmatch(r"[1-9][0-9]*", str(config["macos"]["infoPlist"]["CFBundleVersion"])):
    raise SystemExit("CFBundleVersion must be an integer")
if int(config["macos"]["infoPlist"]["CFBundleVersion"]) < 100:
    raise SystemExit("CFBundleVersion must be at least 100")
config["main"] = str(root)
config["icon"] = str(root / config["icon"])
config["resources"] = [str(root / entry) for entry in config.get("resources", [])]
profile = os.environ.get("RUNWAY_BUILD_PROFILE", "migration-dev")
if profile not in ("migration-dev", "replacement-candidate"):
    raise SystemExit("unknown RUNWAY_BUILD_PROFILE")
development = profile == "migration-dev"
if os.environ.get("V1_DEVELOPMENT_PACKAGE", "1" if development else "0") != ("1" if development else "0"):
    raise SystemExit("development flag conflicts with build profile")
if development:
    config["name"] = "CodexRunway Migration"
    config["identifier"] = "com.github.codex-runway.migration"
    config["urlSchemes"] = ["codex-runway-migration"]
    config["macos"]["infoPlist"]["CFBundleURLTypes"] = [{"CFBundleURLName": "Migration fixtures", "CFBundleURLSchemes": config["urlSchemes"]}]
    config.pop("updates", None)
    config["macos"]["infoPlist"].pop("SUPublicEDKey", None)
else:
    if os.environ.get("V1_UPDATE_CHANNEL", "stable") != "stable":
        raise SystemExit("only the personal stable channel is supported")
    config["updates"]["tagPrefix"] = "personal-v"
    if config["updates"].get("github") != "Klain97710/CodexRunway":
        raise SystemExit("replacement updates must use the personal repository")
config["macos"]["infoPlist"]["RunwayBuildProfile"] = profile
destination.mkdir(parents=True, exist_ok=True)
(destination / "mygo.json").write_text(json.dumps(config, indent=2) + "\n")
