#!/usr/bin/env python3
"""Check cask version, architecture hashes and uninstall data boundaries."""
import hashlib
import importlib.util
import pathlib
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
script = pathlib.Path(__file__).resolve().parents[1] / "generate-v1-homebrew.py"
spec = importlib.util.spec_from_file_location("homebrew", script)
homebrew = importlib.util.module_from_spec(spec)
spec.loader.exec_module(homebrew)


class HomebrewCandidateTests(unittest.TestCase):
    def test_final_dmg_hashes_and_uninstall_boundary(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            for arch in ("arm64", "x86_64"):
                (root / f"Quosparr-macos-{arch}.dmg").write_bytes(arch.encode())
            cask = homebrew.generate(root, {"version": "1.0.0", "identifier": "com.quosparr.app"})
            self.assertIn('version "1.0.0"', cask)
            for arch in ("arm64", "x86_64"):
                self.assertIn(hashlib.sha256(arch.encode()).hexdigest(), cask)
            self.assertIn('Quosparr-macos-#{arch}.dmg', cask)
            self.assertIn('url "https://github.com/Licoy/Quosparr/releases/download/v#{version}/Quosparr-macos-#{arch}.dmg"', cask)
            self.assertIn('verified: "github.com/Licoy/Quosparr/"', cask)
            self.assertIn('homepage "https://github.com/Licoy/Quosparr"', cask)
            self.assertEqual(cask.split("zap trash:")[0].split("uninstall ")[1].strip(), 'quit: "com.quosparr.app"')
            self.assertIn('"~/.quosparr"', cask.split("zap trash:")[1])
            self.assertNotIn('"~/.codex"', cask)
            self.assertNotIn('"~/.grok"', cask)

    def test_beta_and_wrong_identity_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            for config in ({"version": "1.0.0-beta.1", "identifier": "com.quosparr.app"},
                           {"version": "1.0.0", "identifier": "com.quosparr.app.dev"}):
                with self.assertRaises(ValueError):
                    homebrew.generate(root, config)

    def test_missing_or_empty_dmg_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            config = {"version": "1.0.0", "identifier": "com.quosparr.app"}
            with self.assertRaises(FileNotFoundError):
                homebrew.generate(root, config)
            (root / "Quosparr-macos-arm64.dmg").touch()
            with self.assertRaises(ValueError):
                homebrew.generate(root, config)


if __name__ == "__main__":
    unittest.main()
