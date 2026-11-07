from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import shutil

out = Path('docs/rounds/P13-jl')
out.mkdir(exist_ok=True)
names = ['cold-import', 'cold-save-read-error', 'added-neighbour-save', 'added-own-import', 'cold-runtime', 'cold-approval', 'added-neighbour-runtime', 'added-own-approval']
lines = []
for name in names:
    for theme in ['light', 'dark']:
        sources = [Path(f'/tmp/floter-P13-jl-{name}-{theme}-{tag}.png') for tag in ['before', 'after']]
        a, b = [Image.open(p).convert('RGBA') for p in sources]
        count = sum(pa != pb for pa, pb in zip(a.getdata(), b.getdata()))
        lines.append(f'{name} {theme}: changed_pixels={count}')
        assert count > 0, lines[-1]
        for source in sources:
            shutil.copy2(source, out / source.name.removeprefix('floter-P13-jl-'))
        # Preserve native pixels in the comparison; crop only empty lower window.
        board = Image.new('RGB', (1920, 1030), '#888888')
        for x, source, label in [(0, a, 'Before'), (960, b, 'After')]:
            board.paste(source.crop((0, 0, 960, 1000)), (x, 30), source.crop((0, 0, 960, 1000)))
            ImageDraw.Draw(board).text((x + 15, 8), f'{name} / {theme} / {label}', fill='white')
        board.save(out / f'{name}-{theme}-comparison.png')
        if name in ['added-neighbour-save', 'cold-approval']:
            # Inspection crop, nearest neighbour enlargement, not a HiDPI claim.
            crop = (205, 430, 930, 750)
            detail = Image.new('RGB', (2900, 1280), '#888888')
            for x, source in [(0, a), (1450, b)]:
                detail.paste(source.crop(crop).resize((1450, 640), Image.Resampling.NEAREST), (x, 0))
            detail.crop((0, 0, 2900, 640)).save(out / f'{name}-{theme}-detail.png')
(out / 'image-differences.txt').write_text('\n'.join(lines) + '\n')
print('\n'.join(lines))
