from pathlib import Path
from PIL import Image, ImageDraw
import shutil

root = Path(__file__).parent
names = ['label', 'id', 'placeholder', 'default', 'flag', 'options', 'command', 'version']
for theme in ['dark', 'light']:
    panels = []
    for name in names:
        before = Path(f'/tmp/floter-P13-ki-{name}-{theme}-before.png')
        after = Path(f'/tmp/floter-P13-ki-{name}-{theme}-after.png')
        if not before.exists() or not after.exists():
            continue
        for state, src in [('before', before), ('after', after)]:
            shutil.copyfile(src, root / f'{name}-{theme}-{state}.png')
        a, b = Image.open(before).convert('RGB'), Image.open(after).convert('RGB')
        pair = Image.new('RGB', (1440, 550), '#dddddd')
        d = ImageDraw.Draw(pair)
        d.text((12, 8), name + ' BEFORE', fill='black')
        d.text((732, 8), name + ' AFTER', fill='black')
        pair.paste(a, (0, 30)); pair.paste(b, (720, 30))
        pair.save(root / f'{name}-{theme}-comparison.png')
        panels.append(pair.resize((720, 275)))
        detail = pair.crop((170, 75, 1440, 150)).resize((2540, 150), Image.Resampling.NEAREST)
        detail.save(root / f'{name}-{theme}-detail2x.png')
    if panels:
        overview = Image.new('RGB', (1440, 275*((len(panels)+1)//2)), '#dddddd')
        for i, panel in enumerate(panels): overview.paste(panel, ((i%2)*720, (i//2)*275))
        overview.save(root / f'overview-{theme}.png')
