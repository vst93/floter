from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import json, shutil

dest = Path('docs/rounds/P13-kc')
records = []
for before in sorted(Path('/tmp').glob('floter-P13-kc-*-before.png')):
    after = before.with_name(before.name.replace('-before.png', '-after.png'))
    if not after.exists():
        continue
    a, b = Image.open(before).convert('RGB'), Image.open(after).convert('RGB')
    changed = sum(p != (0, 0, 0) for p in ImageChops.difference(a, b).getdata())
    name = before.name.removeprefix('floter-P13-kc-').removesuffix('-before.png')
    canvas = Image.new('RGB', (a.width*2, a.height+30), '#eeeeee')
    canvas.paste(a, (0, 30)); canvas.paste(b, (a.width, 30))
    draw = ImageDraw.Draw(canvas)
    draw.text((12, 10), name+' / BEFORE', fill='black')
    draw.text((a.width+12, 10), 'AFTER', fill='black')
    canvas.save(dest/(name+'-comparison.png'))
    for path in (before, after): shutil.copy2(path, dest/path.name)
    records.append({'surface':name, 'changed_pixels':changed, 'size':a.size})
for theme in ('dark', 'light'):
    pairs = [x for x in records if x['surface'].endswith(theme) and 'only' not in x['surface']]
    canvas = Image.new('RGB', (1440, ((len(pairs)+1)//2)*275), '#eeeeee')
    for i, item in enumerate(pairs):
        pair = Image.open(dest/(item['surface']+'-comparison.png')).resize((720,275))
        canvas.paste(pair, ((i%2)*720, (i//2)*275))
    canvas.save(dest/('overview-'+theme+'.png'))
json.dump(records, open(dest/'image-differences.json', 'w'), indent=2)
print(json.dumps(records, indent=2))
