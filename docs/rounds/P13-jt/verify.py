import hashlib
import json
import os
from pathlib import Path
import subprocess

root = Path('/home/tar/workspace/floter')
out = root / 'docs/rounds/P13-jt'
env = dict(os.environ, LC_ALL='zh_CN.UTF-8', GOMAXPROCS='4')
shell = root / 'internal/shell/shell.go'
core = root / 'internal/extensions/reprobe.go'
files = [shell, core, root/'internal/shell/commandreprobebusy_test.go', root/'internal/shell/commandreprobecurrent_test.go']
original = {p: p.read_bytes() for p in files}
manifest = ''.join(hashlib.md5(data).hexdigest()+'  '+str(p.relative_to(root))+'\n' for p, data in original.items())
(out/'final-source.md5').write_text(manifest)

def run(args, name, extra=None):
    with (out/name).open('w') as log:
        result = subprocess.run(args, cwd=root, env=env if extra is None else dict(env, **extra), stdout=log, stderr=subprocess.STDOUT)
    data = (out/name).read_text()
    return result.returncode, data.count('--- PASS:'), data

def replace_one(text, old, new):
    assert text.count(old) == 1, (old, text.count(old))
    return text.replace(old, new, 1)

def change(which):
    s, c = original[shell].decode(), original[core].decode()
    if which == 'M1':
        start = s.index('func (a *App) reprobeCommands(')
        s = s[:start] + replace_one(s[start:], '!ok || !a.Settings.BeginDiagnosis(id)', '!ok')
    if which == 'M2':
        old = 'if current.ManifestErr != nil || !reflect.DeepEqual(current.Manifest, integration.Manifest) ||\n\t\tdescriptorErr != nil || !bytes.Equal(currentData, data) {'
        c = replace_one(c, old, 'if false && (current.ManifestErr != nil || !reflect.DeepEqual(current.Manifest, integration.Manifest) ||\n\t\tdescriptorErr != nil || !bytes.Equal(currentData, data)) {')
    if which == 'M3':
        c = replace_one(c, 'if current.Entry.PublisherID != integration.Entry.PublisherID ||', 'if false && (current.Entry.PublisherID != integration.Entry.PublisherID ||')
        c = replace_one(c, '!reflect.DeepEqual(current.Entry.RuntimeRoot, integration.Entry.RuntimeRoot) {', '!reflect.DeepEqual(current.Entry.RuntimeRoot, integration.Entry.RuntimeRoot)) {')
    if which == 'M4':
        start = s.index('func (a *App) runCommandReprobe(')
        s = s[:start] + replace_one(s[start:], 'if a.diagnosisReadFailed(integration, epoch) {', 'if false && a.diagnosisReadFailed(integration, epoch) {')
    shell.write_text(s)
    core.write_text(c)

patterns = {
    'M1': ('^TestCommandReprobe(RejectsRepeatedClick|LocksCompetingActions)$', 'pending command reprobe'),
    'M2': ('^TestCommandReprobeKeepsCurrent(Manifest|Descriptor)$', 'late command reprobe overwrote current'),
    'M3': ('^TestCommandReprobeKeepsCurrent(Publisher|Runtime)$', 'late command reprobe overwrote current'),
    'M4': ('^TestCommandReprobeRepository', 'read failure is silent'),
}
results = []
for key, (pattern, expected) in patterns.items():
    try:
        change(key)
        red, _, output = run(['go','test','-count=1','-v','./internal/shell','-run',pattern], key+'-injected-red.txt')
        assert red != 0 and expected in output and '[build failed]' not in output, (key, red, output[-2000:])
    finally:
        for p, data in original.items(): p.write_bytes(data)
        code, _, _ = run(['md5sum','-c',str(out/'final-source.md5')], key+'-restore-md5.txt')
        assert code == 0
        for p in files: p.touch()
    green, count, output = run(['go','test','-count=1','-v','./internal/shell','-run',pattern], key+'-restored-green.txt')
    assert green == 0, (key, output[-2000:])
    results.append(dict(group=key, red_exit=red, restored_exit=green, restored_pass=count))
    print(results[-1], flush=True)
(out/'mutations.json').write_text(json.dumps(results, indent=2)+'\n')

checks = [
    ('gofmt', ['gofmt','-l','cmd','internal'], {}),
    ('vet-linux', ['go','vet','./...'], {'GOOS':'linux'}),
    ('vet-windows', ['go','vet','./...'], {'GOOS':'windows'}),
    ('vet-darwin', ['go','vet','./...'], {'GOOS':'darwin'}),
    ('build-linux', ['go','build','./...'], {'GOOS':'linux'}),
    ('build-windows', ['go','build','./...'], {'GOOS':'windows'}),
    ('build-darwin', ['go','build','./...'], {'GOOS':'darwin'}),
    ('race', ['go','test','-race','-count=1','-v','./internal/shell','-run','^TestCommandReprobe'], {}),
]
summary=[]
for name, args, extra in checks:
    code, count, output = run(args, name+'.txt', extra)
    summary.append(f'{name}: exit={code} PASS={count}')
    (out/'gates.txt').write_text('\n'.join(summary)+'\n')
    print(summary[-1], flush=True)
    assert code == 0 and (name != 'gofmt' or output == ''), output[-2000:]
print('MUTATIONS_AND_STATIC_GATES_PASSED', flush=True)
