import concurrent.futures
import json
import os
from pathlib import Path
import subprocess
import time

root=Path('/home/tar/workspace/floter')
out=root/'docs/rounds/P13-jt'
env=dict(os.environ, LC_ALL='zh_CN.UTF-8', GOMAXPROCS='4')
deadline=time.monotonic()+180
while not (out/'mutations.json').exists():
    if time.monotonic()>deadline: raise RuntimeError('mutations not complete')
    time.sleep(1)
subprocess.run(['md5sum','-c',str(out/'final-source.md5')],cwd=root,check=True)

def check(verbose):
    name='verbose-test.txt' if verbose else 'full-test.txt'
    cmd=['go','test','-count=1']+(['-v'] if verbose else [])+['./...']
    with (out/name).open('w') as log:
        result=subprocess.run(cmd,cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
    data=(out/name).read_text()
    answer=dict(name=name,exit=result.returncode,passed=data.count('--- PASS:'),failed=data.count('--- FAIL:'))
    print(answer,flush=True)
    return answer
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
    results=list(pool.map(check,[False,True]))
(out/'full-results.json').write_text(json.dumps(results,indent=2)+'\n')
assert all(r['exit']==0 for r in results)
