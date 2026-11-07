from pathlib import Path
from PIL import Image, ImageChops, ImageDraw

out = Path(__file__).parent
names = ['plan-command-empty', 'plan-command-restored', 'mode-output', 'mode-platform', 'mode-advanced', 'options-withdrawn', 'remove-last', 'remove-first']
stats = []
for theme in ['light', 'dark']:
    board = Image.new('RGB', (1440, len(names)*192), '#dadada')
    draw = ImageDraw.Draw(board)
    for row, name in enumerate(names):
        pair = []
        for tag in ['before', 'after']:
            source = Path('/tmp') / f'floter-P13-ik-{name}-{theme}-{tag}.png'
            im = Image.open(source).convert('RGB')
            im.save(out / source.name.removeprefix('floter-P13-ik-'))
            pair.append(im)
        diff = ImageChops.difference(*pair)
        count = sum(pixel != (0, 0, 0) for pixel in diff.getdata())
        bounds = diff.getbbox()
        stats.append(f'{name} {theme}: {count} changed pixels; bounds={bounds}')
        assert count > 0, stats[-1]
        center = (bounds[1]+bounds[3])//2
        top = max(0, min(center-80, pair[0].height-160))
        for col, im in enumerate(pair):
            board.paste(im.crop((0, top, 720, top+160)), (col*720, row*192+28))
        draw.text((10, row*192+7), f'{row+1}. {name} / BEFORE (1:1 crop)', fill='black')
        draw.text((730, row*192+7), 'AFTER (1:1 crop)', fill='black')
    board.save(out / f'comparison-{theme}.png')
    # Inspection enlargement, not a high-DPI render.
    board.crop((720, 3*192+28, 1440, 4*192-4)).resize((2880, 640), Image.Resampling.NEAREST).save(out / f'focus-detail-{theme}.png')
(out / 'image-differences.txt').write_text('\n'.join(stats)+'\n')
print('\n'.join(stats))
