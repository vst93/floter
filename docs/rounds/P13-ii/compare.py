from PIL import Image, ImageChops, ImageDraw, ImageFont
from pathlib import Path

root = Path('/home/tar/workspace/floter/docs/rounds/P13-ii')
scenes = ['copy-new-editor', 'copy-unavailable', 'mode-command', 'mode-plan', 'mode-script', 'copy-fold', 'copy-page', 'about-unavailable', 'about-page']
font = ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf', 18)
lines = []
for theme in ['light', 'dark']:
    board = Image.new('RGB', (1480, 210*len(scenes)), '#eeeeee')
    draw = ImageDraw.Draw(board)
    for index, scene in enumerate(scenes):
        before = Image.open(root / f'floter-P13-ii-{scene}-{theme}-before.png').convert('RGB')
        after = Image.open(root / f'floter-P13-ii-{scene}-{theme}-after.png').convert('RGB')
        diff = ImageChops.difference(before, after)
        bounds = diff.getbbox()
        changed = sum(pixel != (0,0,0) for pixel in diff.getdata())
        lines.append(f'{scene} {theme}: changed_pixels={changed}, bounds={bounds}, size={before.size}')
        assert changed > 0, (scene, theme)
        # A 160px context strip around the actual changed control; full frames remain beside it.
        y = max(0, min(before.height-160, ((bounds[1]+bounds[3])//2)-80))
        x = max(0, min(before.width-720, ((bounds[0]+bounds[2])//2)-360))
        crop = (x, y, min(x+720, before.width), y+160)
        top = index*210
        draw.text((10,top+5), f'{index+1}. {scene} / {theme} — BEFORE', fill='#111111', font=font)
        draw.text((750,top+5), 'AFTER', fill='#111111', font=font)
        board.paste(before.crop(crop), (10,top+38))
        board.paste(after.crop(crop), (750,top+38))
    board.save(root / f'comparison-{theme}.png')
(root / 'image-differences.txt').write_text('\n'.join(lines)+'\n')
