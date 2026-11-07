"""Assemble original ui.Tester PNGs; no product/source modifications."""
from pathlib import Path
from PIL import Image, ImageDraw
import shutil

source = Path('/tmp/floter-P13-iy')
dest = Path(__file__).parent
scenes = ['literal-command', 'rekey-command', 'saved-large', 'draft-large',
          'bind-neighbor-conflict', 'failed-conversion-focus']
differences = []
for theme in ['light', 'dark']:
    board = Image.new('RGB', (1440, len(scenes) * 235), '#ececef')
    draw = ImageDraw.Draw(board)
    for i, scene in enumerate(scenes):
        ims = []
        for j, phase in enumerate(['before', 'after']):
            path = source / f'{scene}-{theme}-{phase}.png'
            im = Image.open(path).convert('RGB')
            shutil.copy2(path, dest / path.name)
            ims.append(im)
            top = 700 if scene in ['saved-large', 'draft-large'] else 0
            crop = im.crop((0, top, im.width, min(im.height, top + 210)))
            board.paste(crop, (j * 720, i * 235 + 25))
            draw.text((j * 720 + 12, i * 235 + 6), f'{scene} | {phase}', fill='#15151a')
        differences.append(f'{scene} {theme}: {sum(a != b for a, b in zip(ims[0].getdata(), ims[1].getdata()))} changed pixels')
    board.save(dest / f'comparison-{theme}.png')
    im = Image.open(source / f'draft-large-{theme}-after.png')
    im.crop((195, 748, 702, 864)).resize((2028, 464)).save(dest / f'detail-{theme}.png')
    shutil.copy2(source / f'draft-large-{theme}-after.png', f'/tmp/floter-P13-iy-shortcuts-{theme}.png')
for pattern in ['unknown-*.png', 'bounds-long-key-*.png']:
    for path in source.glob(pattern):
        shutil.copy2(path, dest / path.name)
(dest / 'image-differences.txt').write_text('\n'.join(differences) + '\n')
for path in source.glob('*.log'):
    if path.name in ['baseline.log', 'ordinary.log', 'verbose.log']:
        continue
    shutil.copy2(path, dest / path.name)
