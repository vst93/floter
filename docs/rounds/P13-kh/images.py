from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import shutil

root = Path(__file__).parent
names = ['add', 'remove', 'required', 'kind', 'output', 'save', 'advanced', 'platforms']
differences = []
for appearance in ['light', 'dark']:
    thumbs = []
    for name in names:
        pair = []
        for tag in ['before', 'after']:
            source = Path(f'/tmp/floter-P13-kh-{name}-{appearance}-{tag}.png')
            shutil.copyfile(source, root / f'{name}-{appearance}-{tag}.png')
            pair.append(Image.open(source).convert('RGB'))
        delta = ImageChops.difference(*pair)
        count = sum(p != (0, 0, 0) for p in delta.getdata())
        differences.append(f'{name} {appearance}: {count} changed pixels, bounds={delta.getbbox()}')
        board = Image.new('RGB', (1440, 546), '#cccccc')
        draw = ImageDraw.Draw(board)
        for i, (tag, picture) in enumerate(zip(['before', 'after'], pair)):
            draw.text((i*720+12, 6), f'{name} / {appearance} / {tag}', fill='black')
            board.paste(picture, (i*720, 26))
        board.save(root / f'{name}-{appearance}-comparison.png')
        thumbs.append(board.resize((720, 273)))
        if name in ['add', 'required', 'advanced', 'platforms']:
            # A 4x local view of the bottom edge at its original pixel scale.
            pair[1].crop((480, 470, 720, 520)).resize((960, 200), Image.Resampling.NEAREST).save(root / f'{name}-{appearance}-detail4x.png')
    overview = Image.new('RGB', (1440, 1092), '#cccccc')
    for i, thumb in enumerate(thumbs):
        overview.paste(thumb, ((i%2)*720, (i//2)*273))
    overview.save(root / f'overview-{appearance}.png')
(root / 'image-differences.txt').write_text('\n'.join(differences)+'\n')
