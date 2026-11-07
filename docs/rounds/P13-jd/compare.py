"""Pair actual ui.Tester frames; keep originals at their native resolution."""
from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import shutil

root = Path(__file__).parent
records = []
for before in sorted(Path('/tmp').glob('floter-P13-jd-*-before.png')):
    after = before.with_name(before.name.replace('-before.png', '-after.png'))
    if not after.exists():
        continue
    name = before.name.removeprefix('floter-P13-jd-').removesuffix('-before.png')
    left, right = Image.open(before).convert('RGB'), Image.open(after).convert('RGB')
    assert left.size == right.size, name
    board = Image.new('RGB', (left.width * 2 + 24, left.height + 40), '#dadada')
    draw = ImageDraw.Draw(board)
    draw.text((12, 12), name + ' / BEFORE', fill='#111111')
    draw.text((left.width + 24, 12), 'AFTER', fill='#111111')
    board.paste(left, (0, 40))
    board.paste(right, (left.width + 24, 40))
    board.save(root / (name + '-comparison.png'))
    shutil.copy2(before, root / (name + '-before.png'))
    shutil.copy2(after, root / (name + '-after.png'))
    changed = sum(pixel != (0, 0, 0) for pixel in ImageChops.difference(left, right).getdata())
    records.append(f'{name}: {left.width}x{left.height}; changed_pixels={changed}')
    assert changed > 0, name
(root / 'image-differences.txt').write_text('\n'.join(records) + '\n')
for appearance in ('dark', 'light'):
    shutil.copy2(root / f'report-source-{appearance}-after.png',
                 f'/tmp/floter-P13-jd-transfers-{appearance}.png')
print('\n'.join(records))
