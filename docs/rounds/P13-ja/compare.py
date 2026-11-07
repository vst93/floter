from pathlib import Path
from PIL import Image, ImageChops, ImageDraw

root = Path(__file__).resolve().parent
rows = []
for before in sorted(root.glob('*-before.png')):
    after = before.with_name(before.name.replace('-before.png', '-after.png'))
    if not after.exists():
        continue
    left, right = Image.open(before).convert('RGB'), Image.open(after).convert('RGB')
    diff = ImageChops.difference(left, right)
    count = sum(1 for pixel in diff.getdata() if pixel != (0, 0, 0))
    rows.append(f'{before.stem}: {count} changed pixels; bounds={diff.getbbox()}')
    board = Image.new('RGB', (left.width * 2, left.height + 30), '#eeeeee')
    board.paste(left, (0, 30))
    board.paste(right, (left.width, 30))
    draw = ImageDraw.Draw(board)
    draw.text((12, 8), 'Before: injected old callback', fill='#111111')
    draw.text((left.width + 12, 8), 'After: injected old callback', fill='#111111')
    board.save(before.with_name(before.name.replace('-before.png', '-comparison.png')))
for appearance in ('light', 'dark'):
    im = Image.open(root / f'floter-P13-ja-removed-action-surface-{appearance}-after.png')
    im.crop((180, 610, 950, 820)).resize((1540, 420), Image.Resampling.NEAREST).save(root / f'empty-outlet-{appearance}-2x.png')
(root / 'image-differences.txt').write_text('\n'.join(rows) + '\n')
print('\n'.join(rows))
