"""Archive real Tester frames and readable comparison crops; no new artwork."""
from pathlib import Path
import shutil
from PIL import Image, ImageDraw

root = Path(__file__).resolve().parents[2]
out = root / 'docs/rounds/P13-jv'
out.mkdir(exist_ok=True)
scenarios = ['repeat', 'delete', 'permissions', 'package', 'record', 'removed', 'cancel', 'delete-failure']
for theme in ['light', 'dark']:
    tiles = []
    for scenario in scenarios:
        originals = []
        crops = []
        for tag in ['before', 'after']:
            source = Path(f'/tmp/floter-P13-jv-{scenario}-{theme}-{tag}.png')
            shutil.copy2(source, out / source.name)
            im = Image.open(source).convert('RGB')
            originals.append(im)
            if im.height > 1000:
                crop = Image.new('RGB', (im.width, 520), '#dddddd')
                crop.paste(im.crop((0, 20, im.width, 120)), (0, 0))
                crop.paste(im.crop((0, 760, im.width, 1180)), (0, 100))
            else:
                crop = im.crop((0, max(0, im.height-520), im.width, im.height))
            crops.append(crop.resize((720, 390)))
        comparison = Image.new('RGB', (originals[0].width * 2, originals[0].height + 25), '#bbbbbb')
        draw = ImageDraw.Draw(comparison)
        draw.text((12, 5), f'{scenario} {theme}: BEFORE / AFTER', fill='black')
        comparison.paste(originals[0], (0, 25))
        comparison.paste(originals[1], (originals[0].width, 25))
        comparison.save(out / f'{scenario}-{theme}-compare.png')
        tile = Image.new('RGB', (1440, 420), '#bbbbbb')
        ImageDraw.Draw(tile).text((12, 7), f'{scenario} {theme}: BEFORE / AFTER', fill='black')
        tile.paste(crops[0], (0, 30))
        tile.paste(crops[1], (720, 30))
        tiles.append(tile)
    for half in range(2):
        board = Image.new('RGB', (1440, 1680))
        for index, tile in enumerate(tiles[half*4:half*4+4]):
            board.paste(tile, (0, index*420))
        board.save(out / f'overview-{half}-{theme}.png')
    detail = Image.open(out / f'floter-P13-jv-repeat-{theme}-after.png')
    detail.crop((450, 340, 750, 490)).resize((1200, 600), Image.Resampling.NEAREST).save(out / f'controls-{theme}-4x.png')
print('Archived 32 raw PNGs, 16 comparisons, 4 overview boards and 2 detail crops')
