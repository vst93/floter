from pathlib import Path
from PIL import Image, ImageChops, ImageDraw

out = Path(__file__).parent
names = ['radio', 'checkboxes', 'text', 'number', 'toggle', 'select', 'slider', 'action']
lines = []
for mode in ['light', 'dark']:
    tiles = []
    for name in names:
        for position in ['rest', 'top', 'bottom']:
            images = []
            for phase in ['before', 'after']:
                source = Path(f'/tmp/floter-P13-kn-{name}-{position}-{mode}-{phase}.png')
                image = Image.open(source).convert('RGB')
                image.save(out / f'{name}-{position}-{mode}-{phase}.png')
                images.append(image)
            diff = ImageChops.difference(*images)
            count = sum(pixel != (0, 0, 0) for pixel in diff.getdata())
            lines.append(f'{name}-{position}-{mode}: {count} changed pixels')
            if position == 'rest':
                assert count == 0, f'{name} resting pixels changed'
                continue
            assert count > 0, f'{name} missing visual change'
            pair = Image.new('RGB', (1120, 286), '#898989')
            pair.paste(images[0], (0, 26)); pair.paste(images[1], (560, 26))
            ImageDraw.Draw(pair).text((12, 8), f'{name} {position} {mode} | BEFORE', fill='white')
            ImageDraw.Draw(pair).text((572, 8), 'AFTER', fill='white')
            pair.save(out / f'{name}-{position}-{mode}-comparison.png')
            if position == 'top':
                tiles.append(pair.resize((560, 143)))
                images[1].crop((20, 0, 540, 65)).resize((2080, 260), Image.Resampling.NEAREST).save(out / f'{name}-{mode}-detail4x.png')
    board = Image.new('RGB', (1120, 572), '#898989')
    for i, tile in enumerate(tiles): board.paste(tile, ((i % 2)*560, (i // 2)*143))
    board.save(out / f'overview-{mode}.png')
(out / 'image-differences.txt').write_text('\n'.join(lines)+'\n')
print('\n'.join(lines))
