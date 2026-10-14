#!/usr/bin/env python3
import importlib.util
import pathlib
import unittest
import tempfile
import subprocess
import os
import json

SCRIPT=pathlib.Path(__file__).resolve().parents[1]/"check-v1-build-number.py"
spec=importlib.util.spec_from_file_location("builds",SCRIPT)
builds=importlib.util.module_from_spec(spec)
spec.loader.exec_module(builds)

def feed(version):
 return f'<rss xmlns:sparkle="{builds.NS}"><channel><item><enclosure sparkle:version="{version}"/></item></channel></rss>'.encode()

class BuildNumberTests(unittest.TestCase):
 def test_candidate_exceeds_all_architectures(self):
  self.assertEqual(builds.check("100",[feed("87"),feed("40.1234.5")]),(87,))
 def test_historical_or_bad_build_rejected(self):
  for candidate,feeds in (("99",[feed("87")]),("100",[feed("100")]),("101",[feed("102")]),("1.0.0",[feed("87")]),("100",[]),("100",[feed("unknown")])):
   with self.assertRaises(ValueError): builds.check(candidate,feeds)

class StageConfigTests(unittest.TestCase):
 def test_stable_uses_personal_repository(self):
  root=pathlib.Path(__file__).resolve().parents[2]
  with tempfile.TemporaryDirectory(prefix="quosparr-config-test-") as temporary:
   env=os.environ.copy()
   env.update(V1_CONFIG_PATH=str(root/"mygo.json"),V1_UPDATE_CHANNEL="stable",V1_DEVELOPMENT_PACKAGE="0",RUNWAY_BUILD_PROFILE="replacement-candidate")
   subprocess.run(["python3",str(root/"Scripts/stage-v1-config.py"),str(root),temporary],env=env,check=True)
   staged=json.loads((pathlib.Path(temporary)/"mygo.json").read_text())
   self.assertEqual(staged["updates"]["github"],"Klain97710/CodexRunway")
   self.assertEqual(staged["updates"]["tagPrefix"],"personal-v")
   self.assertNotIn("url",staged["updates"])

 def test_rename_notice_is_disabled(self):
  root=pathlib.Path(__file__).resolve().parents[2]
  result=subprocess.run(["python3",str(root/"Scripts/legacy-install-notice.py")],capture_output=True)
  self.assertNotEqual(result.returncode,0)
  self.assertIn(b"signed XML and ZIP",result.stderr)

if __name__=="__main__": unittest.main()
