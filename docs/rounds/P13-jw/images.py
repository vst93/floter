from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import shutil

out = Path('/home/tar/workspace/floter/docs/rounds/P13-jw')
scenarios = ['local-busy', 'recommended-busy', 'local-permissions', 'recommended-permissions', 'local-package', 'recommended-package', 'link-cancel', 'registry-cancel']
lines = []
for scenario in scenarios:
    for theme in ['light', 'dark']:
        images = []
        for tag in ['before', 'after']:
            source = Path(f'/tmp/floter-P13-jw-{scenario}-{theme}-{tag}.png')
            shutil.copy2(source, out / f'{scenario}-{theme}-{tag}.png')
            images.append(Image.open(source).convert('RGB'))
        before, after = images
        delta = ImageChops.difference(before, after)
        changed = sum(pixel != (0, 0, 0) for pixel in delta.getdata())
        lines.append(f'{scenario} {theme}: {changed} changed pixels; bbox={delta.getbbox()}')
        canvas = Image.new('RGB', (1920, 1640), '#d8d8d8')
        canvas.paste(before, (0, 40)); canvas.paste(after, (960, 40))
        draw = ImageDraw.Draw(canvas)
        draw.text((20, 12), scenario + ' / ' + theme + ' / BEFORE', fill='black')
        draw.text((980, 12), 'AFTER', fill='black')
        canvas.save(out / f'{scenario}-{theme}-comparison.png')
        if scenario in ['local-busy', 'recommended-busy', 'registry-cancel']:
            # Inspection enlargement only; these are not native high-DPI captures.
            box = {'local-busy':(770,825,935,905),'recommended-busy':(770,620,935,705),'registry-cancel':(610,40,950,125)}[scenario]
            detail = after.crop(box)
            detail.resize((detail.width*4,detail.height*4),Image.Resampling.NEAREST).save(out/f'{scenario}-{theme}-detail4x.png')
(out / 'image-differences.txt').write_text('\n'.join(lines)+'\n')
print('\n'.join(lines))

for theme in ['light', 'dark']:
    board = Image.new('RGB', (1920, 820), '#d8d8d8')
    for i, scenario in enumerate(scenarios):
        comparison = Image.open(out / f'{scenario}-{theme}-comparison.png')
        comparison.thumbnail((480, 410))
        x, y = (i % 4) * 480, (i // 4) * 410
        board.paste(comparison, (x, y))
    board.save(out / f'overview-{theme}.png')
