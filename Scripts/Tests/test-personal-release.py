#!/usr/bin/env python3
"""Real Sparkle signature checks using a public RFC 8032 test key, never user keys."""
import base64
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]

class PersonalReleaseTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tool = subprocess.check_output(['bash', str(ROOT/'Scripts/prepare-personal-sparkle-tools.sh')], text=True).strip()

    def test_signed_zip_does_not_make_unsigned_or_tampered_xml_valid(self):
        # RFC 8032 test vector: intentionally public and unrelated to release keys.
        seed = base64.b64encode(bytes.fromhex('9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60')).decode()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            key = root/'test.key'; key.write_text(seed); key.chmod(0o600)
            archive = root/'test.zip'; archive.write_bytes(b'synthetic archive bytes')
            def run(*args):
                return subprocess.run([self.tool, '--ed-key-file', str(key), *map(str,args)],capture_output=True,text=True)
            signed = run('-p',archive)
            self.assertEqual(signed.returncode,0,signed.stderr)
            signature = signed.stdout.strip()
            self.assertEqual(run('--verify',archive,signature).returncode,0)
            feed=root/'appcast.xml'
            original=f'<rss xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle" version="2.0"><channel><title>Test</title><item><sparkle:version>2000</sparkle:version><enclosure url="https://github.com/Klain97710/CodexRunway/releases/download/personal-v0.2.0/test.zip" sparkle:edSignature="{signature}" length="{archive.stat().st_size}" type="application/octet-stream"/></item></channel></rss>'
            feed.write_text(original)
            self.assertNotEqual(run('--verify',feed).returncode,0,'Unsigned XML accepted despite signed ZIP')
            result=run(feed); self.assertEqual(result.returncode,0,result.stderr)
            self.assertEqual(run('--verify',feed).returncode,0)
            feed.write_text(feed.read_text().replace('<title>Test</title>','<title>Tampered</title>'))
            self.assertNotEqual(run('--verify',feed).returncode,0,'Tampered XML accepted')
            archive.write_bytes(b'truncated')
            self.assertNotEqual(run('--verify',archive,signature).returncode,0,'Corrupt ZIP accepted')

    def test_candidate_requires_both_valid_keys_before_build(self):
        env={k:v for k,v in os.environ.items() if k not in ('SPARKLE_PRIVATE_KEY','MYGO_UPDATE_PRIVATE_KEY','MYGO_UPDATER_PRIVATE_KEY')}
        result=subprocess.run(['go','run','./Scripts/release-assets','check-keys','-config','mygo.json'],cwd=ROOT,env=env,capture_output=True)
        self.assertNotEqual(result.returncode,0)

    def test_identity_and_channel(self):
        cfg=json.loads((ROOT/'mygo.json').read_text())
        self.assertEqual(cfg['name'],'CodexRunway')
        self.assertEqual(cfg['identifier'],'com.github.codex-runway')
        self.assertEqual(cfg['updates']['github'],'Klain97710/CodexRunway')
        self.assertEqual(cfg['updates']['tagPrefix'],'personal-v')
        self.assertEqual(len(base64.b64decode(cfg['updates']['publicKey'])),32)
        self.assertNotEqual(cfg['updates']['publicKey'],cfg['macos']['infoPlist']['SUPublicEDKey'])
        self.assertNotIn('privateKey',cfg['updates'])
        self.assertNotIn('url',cfg['updates'])

if __name__=='__main__': unittest.main()
