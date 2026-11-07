"""Build review sheets from the actual ui.Tester PNGs (Pillow, no product dependency)."""
from pathlib import Path
from shutil import copy2
from PIL import Image, ImageChops, ImageDraw

source = Path('/tmp/floter-P13-jc')
dest = Path(__file__).resolve().parent
names = ['fallback-refused', 'fallback-recorded', 'saved-menu-select', 'saved-menu-escape',
         'draft-menu-select', 'draft-menu-escape', 'saved-key-tip', 'draft-key-tip',
         'upper-menu-saved-recorder', 'upper-menu-draft-recorder']
stats = []
for appearance in ['light', 'dark']:
    thumbs = []
    for name in names:
        pair = []
        for stage in ['before', 'after']:
            filename = f'{name}-{appearance}-{stage}.png'
            copy2(source / filename, dest / filename)
            pair.append(Image.open(dest / filename).convert('RGB'))
        if pair[0].size != pair[1].size:
            raise ValueError(name + ': unequal frame sizes')
        diff = ImageChops.difference(*pair)
        changed = sum(pixel != (0, 0, 0) for pixel in diff.getdata())
        stats.append(f'{name}-{appearance}: {changed} changed pixels; {pair[0].size}')
        if changed == 0:
            raise ValueError(name + ': empty comparison')
        w, h = pair[0].size
        board = Image.new('RGB', (2*w+12, h+28), '#dddddd')
        draw = ImageDraw.Draw(board)
        draw.text((8,8), name+' / BEFORE', fill='black')
        draw.text((w+20,8), name+' / AFTER', fill='black')
        board.paste(pair[0], (0,28)); board.paste(pair[1], (w+12,28))
        board.save(dest/f'{name}-{appearance}-comparison.png')
        thumb = board.copy(); thumb.thumbnail((960, 580))
        thumbs.append(thumb)
    overview = Image.new('RGB', (960, sum(i.height+12 for i in thumbs)), '#bbbbbb')
    y=0
    for thumb in thumbs:
        overview.paste(thumb,(0,y)); y+=thumb.height+12
    overview.save(dest/f'overview-{appearance}.png')
    original = Image.open(dest/f'draft-key-tip-{appearance}-after.png')
    original.crop((145,710,705,940)).resize((2240,920), Image.Resampling.NEAREST).save(dest/f'tip-detail-{appearance}.png')
    copy2(dest/f'draft-key-tip-{appearance}-after.png',f'/tmp/floter-P13-jc-shortcuts-{appearance}.png')
(dest/'image-differences.txt').write_text('\n'.join(stats)+'\n')
print('\n'.join(stats))
