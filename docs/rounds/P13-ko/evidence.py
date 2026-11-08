from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import json
import shutil

dest = Path('/home/tar/workspace/floter/docs/rounds/P13-ko')
dest.mkdir(exist_ok=True)
root = Path('/tmp')
kinds = ['radio', 'checkboxes', 'text', 'number', 'toggle', 'select', 'slider', 'action']
diffs = {}
for appearance in ['light', 'dark']:
    overview = Image.new('RGB', (1120, 8 * 286), '#888888')
    for row, kind in enumerate(kinds):
        for stage in ['rest', 'failure', 'heading', 'recovery']:
            images = []
            for version in ['before', 'after']:
                source = root / f'floter-P13-ko-{kind}-{stage}-{appearance}-{version}.png'
                shutil.copy2(source, dest / source.name.removeprefix('floter-P13-ko-'))
                images.append(Image.open(source).convert('RGB'))
            delta = ImageChops.difference(*images)
            count = sum(pixel != (0, 0, 0) for pixel in delta.getdata())
            key = f'{kind}-{stage}-{appearance}'
            diffs[key] = count
            if stage == 'rest':
                assert count == 0, (key, count)
            else:
                assert count > 0, (key, count)
            panel = Image.new('RGB', (1120, 286), '#888888')
            draw = ImageDraw.Draw(panel)
            draw.text((10, 7), f'{kind} / {stage} / {appearance} / BEFORE', fill='white')
            draw.text((570, 7), 'AFTER', fill='white')
            panel.paste(images[0], (0, 26))
            panel.paste(images[1], (560, 26))
            if stage != 'rest':
                panel.save(dest / f'{key}-comparison.png')
            if stage == 'failure':
                overview.paste(panel, (0, row * 286))
                # Nearest-neighbour enlargement for seam inspection, not a HiDPI render.
                for version, image in zip(['before', 'after'], images):
                    image.crop((0, 196, 560, 260)).resize((2240, 256), Image.Resampling.NEAREST).save(dest / f'{kind}-{appearance}-{version}-4x.png')
    overview.save(dest / f'overview-{appearance}.png')
for source in root.glob('floter-P13-ko-wrapped-*-after.png'):
    shutil.copy2(source, dest / source.name.removeprefix('floter-P13-ko-'))
(dest / 'image-differences.json').write_text(json.dumps(diffs, indent=2) + '\n')
print(f'{len(diffs)} comparisons; {sum(v == 0 for v in diffs.values())} unchanged resting frames; {min(v for v in diffs.values() if v)} minimum changed pixels')
