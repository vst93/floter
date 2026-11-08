"""P13-jv reproducible, bounded mutations. Restores bytes even on failure."""
import hashlib
import os
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parents[2]
out = Path('/tmp/floter-P13-jv')
groups = {
    'M1': ('internal/settingsui/settingsui.go', [
        ('if a.orphanOperation[integration.ID] == "adopt" {', 'if false {'),
        ('if a.orphanOperation[integration.ID] == "delete" {', 'if false {'),
        ('} else if a.orphanOperation[integration.ID] != "" {', '} else if false {'),
    ], './internal/settingsui', '^TestOrphanBusy', 'pending orphan permits'),
    'M2': ('internal/extensions/install.go', [
        ('if p.adopting {', 'if false && p.adopting {'),
    ], './internal/shell', '^TestAdoptionCurrent(Permissions|Package)$', 'adoption rewrote current repository'),
    'M3': ('internal/extensions/install.go', [
        ('if _, err := orphanDir(p.paths, p.Manifest.ID); err != nil {', 'if _, err := orphanDir(p.paths, p.Manifest.ID); false && err != nil {'),
    ], './internal/shell', '^TestAdoptionCurrent(Record|Removed)$', 'adoption'),
    'M4': ('internal/shell/shell.go', [
        ('if !approved {\n\t\t\t\t\ta.RefreshIntegrations(context.Background())', 'if false && !approved {\n\t\t\t\t\ta.RefreshIntegrations(context.Background())'),
        ('a.notifyFailure(func(c i18n.Notifications) string { return c.IntegrationRemoveFailed(id) })', '// mutation: missing deletion failure notification'),
    ], './internal/shell', '^TestAdoption(Cancel|DeleteFailure)$', 'result'),
}
env = dict(os.environ, LC_ALL='zh_CN.UTF-8', GOMAXPROCS='4')
for name in sys.argv[1:]:
    filename, changes, package, pattern, failure = groups[name]
    path = root / filename
    original = path.read_bytes()
    checksum = out / (name + '.md5')
    checksum.write_text(hashlib.md5(original).hexdigest() + '  ' + str(path) + '\n')
    modified = original.decode()
    for old, new in changes:
        assert old in modified, (name, 'missing mutation target', old)
        modified = modified.replace(old, new)
    command = ['go', 'test', '-count=1', '-v', package, '-run', pattern]
    try:
        path.write_text(modified)
        with (out / (name + '-red.log')).open('w') as log:
            red = subprocess.run(command, cwd=root, env=dict(env, FLOTER_P13JV_DUMP='before'), stdout=log, stderr=subprocess.STDOUT)
        text = (out / (name + '-red.log')).read_text()
        assert red.returncode != 0 and '--- FAIL' in text and failure in text, (name, 'mutation did not fail meaningfully', text[-2000:])
    finally:
        path.write_bytes(original)
        subprocess.run(['md5sum', '-c', str(checksum)], check=True)
        path.touch()
    with (out / (name + '-restored.log')).open('w') as log:
        green = subprocess.run(command, cwd=root, env=dict(env, FLOTER_P13JV_DUMP='after'), stdout=log, stderr=subprocess.STDOUT)
    assert green.returncode == 0, (name, 'restored test failed')
    count = (out / (name + '-restored.log')).read_text().count('--- PASS')
    print(f'{name}: red exit={red.returncode}, MD5 restored, touch retest {count} PASS', flush=True)
