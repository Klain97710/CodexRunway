#!/usr/bin/env python3
"""Exercise the real workflow's release command with an offline gh recorder."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[2]

def workflow(path):
    return json.loads(subprocess.check_output(['ruby','-rjson','-ryaml','-e','puts JSON.generate(YAML.safe_load(File.read(ARGV.fetch(0))))',str(path)],text=True))

class DraftWorkflowTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflows={p.name:workflow(p) for p in (ROOT/'.github/workflows').glob('*.yml')}

    def test_create_command_and_failure(self):
        steps=self.workflows['release.yml']['jobs']['candidate']['steps']
        commands=[s['run'] for s in steps if 'gh release create' in s.get('run','')]
        self.assertEqual(len(commands),1)
        for exit_code in (0,23):
            with tempfile.TemporaryDirectory() as temp:
                root=Path(temp);(root/'dist-v1').mkdir();(root/'dist-v1/test.zip').write_bytes(b'fixture')
                (root/'release-notes.md').write_text('Candidate')
                (root/'gh').write_text('#!/usr/bin/env python3\nimport os,json,sys\nopen("calls.json","w").write(json.dumps(sys.argv[1:]))\nsys.exit(int(os.environ["FAKE_EXIT"]))\n')
                (root/'gh').chmod(0o755)
                env=dict(os.environ,PATH=str(root)+os.pathsep+os.environ['PATH'],GITHUB_REF_NAME='personal-v0.2.0',GITHUB_REPOSITORY='Klain97710/CodexRunway',FAKE_EXIT=str(exit_code),BASH_ENV='/dev/null')
                result=subprocess.run(['bash','--noprofile','--norc','-euo','pipefail','-c',commands[0]],cwd=root,env=env,capture_output=True)
                self.assertEqual(result.returncode,exit_code)
                args=json.loads((root/'calls.json').read_text())
                self.assertEqual(args[:3],['release','create','personal-v0.2.0'])
                for flag in ('--draft','--latest=false','--verify-tag','--notes-file'): self.assertIn(flag,args)
                self.assertIn('dist-v1/test.zip',args)

    def test_triggers_permissions_and_no_publication(self):
        release=self.workflows['release.yml']
        # Psych follows YAML 1.1 and parses the GitHub `on` key as boolean true.
        self.assertEqual(release.get('on',release.get('true'))['push']['tags'],['personal-v*'])
        for name,w in self.workflows.items():
            self.assertEqual(w['permissions'],{'contents':'read'})
            for job_id,job in w['jobs'].items():
                write=job.get('permissions',{}).get('contents')=='write'
                self.assertEqual(write,name=='release.yml' and job_id=='candidate')
                for step in job.get('steps',[]):
                    command=step.get('run','')
                    for text in ('gh release edit','--draft=false','--latest=true','gh release upload','--clobber'):
                        self.assertNotIn(text,command)
        self.assertIn('refs/tags/personal-v',release['jobs']['candidate']['if'])
        self.assertNotIn('secrets.',json.dumps(self.workflows['ci.yml']))

if __name__=='__main__': unittest.main()
