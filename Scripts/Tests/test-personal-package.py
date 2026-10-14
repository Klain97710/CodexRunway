#!/usr/bin/env python3
"""Profile and sandbox regression tests; no application launch or credentials."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('migration_runner', ROOT / 'Scripts/run-personal-migration.py')
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)

macho_spec=importlib.util.spec_from_file_location('personal_macho',ROOT/'Scripts/verify-personal-macho.py')
macho=importlib.util.module_from_spec(macho_spec)
macho_spec.loader.exec_module(macho)


class Profiles(unittest.TestCase):
    def test_macho_rejects_false_minimum_os_claim(self):
        with self.assertRaises(ValueError):
            macho.check_minimum('    minos 13.0\n      sdk 26.2\n','12.0')
        macho.check_minimum('    minos 13.0\n','13.0')
        macho.check_minimum('    minos 12.0\n','13.0')
        with self.assertRaises(ValueError): macho.check_minimum('sdk 12.0','12.0')

    def stage(self, env):
        with tempfile.TemporaryDirectory() as temp:
            base = {k: v for k, v in os.environ.items() if not k.startswith(('RUNWAY_', 'V1_'))}
            result = subprocess.run(['python3', str(ROOT / 'Scripts/stage-v1-config.py'), str(ROOT), temp], env=base | env, capture_output=True)
            config = Path(temp) / 'mygo.json'
            return result, json.loads(config.read_text()) if config.exists() else None

    def test_default_is_isolated_without_update_keys(self):
        result, cfg = self.stage({})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(cfg['identifier'], 'com.github.codex-runway.migration')
        self.assertEqual(cfg['name'], 'CodexRunway Migration')
        self.assertNotIn('updates', cfg)
        self.assertNotIn('SUPublicEDKey', cfg['macos']['infoPlist'])
        self.assertEqual(cfg['urlSchemes'], ['codex-runway-migration'])

    def test_conflicting_flags_rejected(self):
        result, _ = self.stage({'V1_DEVELOPMENT_PACKAGE': '0'})
        self.assertNotEqual(result.returncode, 0)

    def test_sandbox_rejects_unmarked_and_linked_data(self):
        with tempfile.TemporaryDirectory() as temp, tempfile.TemporaryDirectory() as outside:
            root = Path(temp).resolve()
            with self.assertRaises((ValueError, FileNotFoundError)):
                runner.validate(root)
            (root / runner.MARKER).write_text(runner.MARKER_VALUE)
            self.assertEqual(runner.validate(root), root)
            (root / 'escape').symlink_to(outside)
            with self.assertRaises(ValueError):
                runner.validate(root)

    def test_init_refuses_to_mark_existing_data(self):
        with tempfile.TemporaryDirectory() as temp:
            result = subprocess.run(['python3', str(ROOT / 'Scripts/run-personal-migration.py'), '--sandbox', temp, '--init'], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse((Path(temp) / runner.MARKER).exists())


if __name__ == '__main__':
    unittest.main()
