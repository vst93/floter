from pathlib import Path
from PIL import Image, ImageDraw, ImageChops

root = Path('/home/tar/workspace/floter/docs/rounds/P13-ih')
root.mkdir(parents=True, exist_ok=True)
scenes = ['fetch-new', 'fetch-empty', 'picker-new', 'picker-closed', 'save-new', 'connect-new', 'lookup-new', 'lookup-retired']
stats = []
for theme in ['light', 'dark']:
    # Comparison board uses matching top regions at native pixel size. Full
    # frames remain alongside it, including off-board fields and actions.
    canvas = Image.new('RGB', (1440, 8 * 452), '#dcdcdc' if theme == 'light' else '#202124')
    draw = ImageDraw.Draw(canvas)
    for i, scene in enumerate(scenes):
        frames = []
        for j, tag in enumerate(['before', 'after']):
            path = Path(f'/tmp/floter-P13-ih-{scene}-{theme}-{tag}.png')
            img = Image.open(path).convert('RGB')
            img.save(root / path.name)
            frames.append(img)
            y = i * 452
            draw.text((j * 720 + 12, y + 8), f'{i+1}. {scene} / {tag}', fill='black' if theme == 'light' else 'white')
            canvas.paste(img.crop((0, 0, 720, 420)), (j * 720, y + 28))
        diff = ImageChops.difference(*frames)
        changed = sum(px != (0,0,0) for px in diff.getdata())
        stats.append(f'{theme} {scene}: changed={changed}, bbox={diff.getbbox()}, size={frames[0].size}')
    canvas.save(root / f'comparison-{theme}.png')
(root / 'image-differences.txt').write_text('\n'.join(stats) + '\n')
print('\n'.join(stats))
