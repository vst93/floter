from pathlib import Path
from shutil import copy2
from PIL import Image, ImageChops, ImageDraw

root = Path(__file__).parent
names = ['filesystem-read', 'filesystem-write', 'network-fetch', 'process-spawn',
         'clipboard-read', 'clipboard-write', 'environment', 'script']
differences = []
for theme in ['light', 'dark']:
    overview = Image.new('RGB', (720, 280 * len(names)), '#dddddd')
    for i, name in enumerate(names):
        pictures = []
        for stage in ['before', 'after']:
            source = Path(f'/tmp/floter-P13-kk-{name}-{theme}-{stage}.png')
            copy2(source, root / source.name.removeprefix('floter-P13-kk-'))
            pictures.append(Image.open(source).convert('RGB'))
        before, after = pictures
        count = sum(any(pixel) for pixel in ImageChops.difference(before, after).getdata())
        assert count > 0, (name, theme)
        differences.append(f'{name} {theme}: {count} changed pixels')
        pair = Image.new('RGB', (1440, 560), '#dddddd')
        pair.paste(before, (0, 40)); pair.paste(after, (720, 40))
        draw = ImageDraw.Draw(pair)
        draw.text((12, 12), f'{name} {theme} BEFORE', fill='black')
        draw.text((732, 12), f'{name} {theme} AFTER', fill='black')
        pair.save(root / f'{name}-{theme}-comparison.png')
        overview.paste(pair.resize((720, 280)), (0, 280*i))
        if name in ['filesystem-read', 'environment', 'script']:
            box = (195, 45, 690, 150) if name == 'script' else (620, 45, 700, 110)
            parts = [im.crop(box).resize(((box[2]-box[0])*4, (box[3]-box[1])*4), Image.Resampling.NEAREST) for im in pictures]
            crop = Image.new('RGB', (parts[0].width*2, parts[0].height))
            crop.paste(parts[0], (0,0)); crop.paste(parts[1], (parts[0].width,0))
            crop.save(root / f'{name}-{theme}-detail4x.png')
    overview.save(root / f'overview-{theme}.png')
(root / 'image-differences.txt').write_text('\n'.join(differences)+'\n')

rest = []
for before in sorted(Path('/tmp').glob('floter-P13-kk-rest-*-before.*')):
    after = Path(str(before).replace('-before.', '-after.'))
    assert before.read_bytes() == after.read_bytes(), before.name
    rest.append(f'IDENTICAL {before.name} {after.name}')
    copy2(before, root / before.name.removeprefix('floter-P13-kk-'))
    copy2(after, root / after.name.removeprefix('floter-P13-kk-'))
assert len(rest) == 24, len(rest)
(root / 'rest-comparison.txt').write_text('\n'.join(rest)+'\n')
print('\n'.join(differences))
print(f'{len(rest)} resting PNG/geometry pairs byte identical')
