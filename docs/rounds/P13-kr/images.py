from pathlib import Path
from PIL import Image, ImageChops, ImageDraw

out = Path(__file__).parent
records = []
for before in sorted(Path('/tmp').glob('floter-P13-kr-*-before.png')):
    after = before.with_name(before.name.replace('-before.png', '-after.png'))
    if not after.exists():
        raise RuntimeError(f'Missing after: {after}')
    name = before.name.removeprefix('floter-P13-kr-').removesuffix('-before.png')
    a, b = Image.open(before).convert('RGB'), Image.open(after).convert('RGB')
    a.save(out / f'{name}-before.png')
    b.save(out / f'{name}-after.png')
    bounds = ImageChops.difference(a, b).getbbox()
    records.append(f'{name}: {bounds}')
    if '-change0-' in name:
        sheet = Image.new('RGB', (1440, 544), '#777777')
        sheet.paste(a, (0, 24)); sheet.paste(b, (720, 24))
        draw = ImageDraw.Draw(sheet)
        draw.text((12, 5), 'BEFORE', fill='white')
        draw.text((732, 5), 'AFTER', fill='white')
        sheet.save(out / f'{name}-comparison.png')
(out / 'image-differences.txt').write_text('\n'.join(records) + '\n')

scenarios = ['run-text', 'run-boolean', 'run-select', 'run-cancel',
             'config-text', 'config-defaults', 'config-save', 'config-copy']
for appearance in ['dark', 'light']:
    overview = Image.new('RGB', (1440, 8 * 272), '#777777')
    for row, scenario in enumerate(scenarios):
        name = f'{scenario}-change0-{appearance}'
        before = Image.open(out / f'{name}-before.png').convert('RGB')
        after = Image.open(out / f'{name}-after.png').convert('RGB')
        # The lower part contains the active outlet; preserve it at 1x.
        overview.paste(before.crop((0, 272, 720, 520)), (0, row * 272 + 24))
        overview.paste(after.crop((0, 272, 720, 520)), (720, row * 272 + 24))
        ImageDraw.Draw(overview).text((12, row * 272 + 5), scenario + ' / BEFORE                          AFTER', fill='white')
    overview.save(out / f'overview-{appearance}.png')
    detail = Image.open(out / f'config-save-change0-{appearance}-after.png')
    detail.crop((560, 462, 710, 510)).resize((600, 192)).save(out / f'save-detail4x-{appearance}.png')
