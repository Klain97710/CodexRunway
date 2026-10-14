#!/usr/bin/env python3
"""Run the fixed migration gate; preserve failures and report optional skips."""
import json
import os
from pathlib import Path
import subprocess
import time

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'build/regressions'
OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,USAGE_REQUIRE_SQLITE='1')
# Opt-in desktop/Keychain probes belong to the disposable acceptance environment.
for name in ('RUNWAY_NATIVE_PANEL_HOST','QUOSPARR_KEYCHAIN_TEST','QUOSPARR_TEST_DESKTOP_FAILURE'):
    env.pop(name,None)
commands=[
 ('go-test',['go','test','-json','-count=1','-timeout=10m','./...']),
 ('go-vet',['go','vet','./...']),
 ('go-build',['go','build','./...']),
 ('go-race',['go','test','-json','-race','-count=1','-timeout=10m','./internal/...']),
 ('development-update',['go','test','-json','-tags=quosparr_dev_package','-count=1','./internal/app','-run','^TestPackagedDevelopmentUpdate']),
 ('personal-package',['python3','Scripts/Tests/test-personal-package.py']),
 ('personal-release',['python3','Scripts/Tests/test-personal-release.py']),
 ('draft-workflow',['python3','Scripts/Tests/test-workflow-drafts.py']),
 ('historical-release',['python3','Scripts/Tests/test-v1-release.py']),
]
report={'source':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),
        'dirty':bool(subprocess.check_output(['git','status','--porcelain'],cwd=ROOT,text=True)),
        'checks':[]}
for name,command in commands:
    print('RUN',name,flush=True)
    start=time.monotonic()
    with (OUT/(name+'.log')).open('w') as log:
        try:
            result=subprocess.run(command,cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT,timeout=900)
            code=result.returncode
        except subprocess.TimeoutExpired:
            code=124
    entry={'name':name,'command':command,'exitCode':code,'seconds':round(time.monotonic()-start,2)}
    if '-json' in command:
        events=[]
        for line in (OUT/(name+'.log')).read_text().splitlines():
            try: events.append(json.loads(line))
            except json.JSONDecodeError: pass
        entry['passed']=sum(e.get('Action')=='pass' and 'Test' in e for e in events)
        entry['skipped']=[e['Package']+'/'+e['Test'] for e in events if e.get('Action')=='skip' and 'Test' in e]
        entry['failed']=[e.get('Package','')+'/'+e.get('Test','') for e in events if e.get('Action')=='fail']
    report['checks'].append(entry)
    (OUT/'report.json').write_text(json.dumps(report,indent=2)+'\n')
    print('PASS' if code==0 else 'FAIL',name,'log:',OUT/(name+'.log'),flush=True)
    if code: raise SystemExit(code)
print('All automated checks passed. Skipped environment/preview probes remain NOT RUN; see report.json.',flush=True)
