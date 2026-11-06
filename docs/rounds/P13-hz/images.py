from PIL import Image, ImageChops, ImageDraw, ImageFont
from pathlib import Path
import shutil

target = Path('docs/rounds/P13-hz')
names = ['discard-owner','discard-orphan','discard-schema','discard-unopenable',
         'disable-no-adopt','remove-no-adopt','uninstall-no-adopt','disable-no-check']
lines = []
for theme in ['light', 'dark']:
    board = Image.new('RGB', (1440, 8*570), '#d5d5d5')
    draw = ImageDraw.Draw(board)
    for row, name in enumerate(names):
        pair = []
        for col, state in enumerate(['before','after']):
            source = Path(f'/tmp/floter-P13-hz-{name}-{theme}-{state}.png')
            shutil.copy2(source, target/source.name)
            im = Image.open(source).convert('RGB')
            pair.append(im)
            # Whole controls occupy the top 540px; originals retain all 900px.
            board.paste(im.crop((0,0,720,540)), (720*col,570*row+30))
            draw.text((720*col+12,570*row+8),f'{row+1}. {name} / {state}', fill='black')
        changed = sum(1 for p in ImageChops.difference(*pair).getdata() if p != (0,0,0))
        lines.append(f'{name} {theme}: {changed} changed pixels')
        if changed == 0:
            raise RuntimeError(lines[-1])
    board.save(target/f'comparison-{theme}.png')
    # Inspect one row outlet and one header outlet without shrinking glyphs.
    detail = Image.new('RGB',(1440,960),'#d5d5d5')
    for col, state in enumerate(['before','after']):
        im = Image.open(target/f'floter-P13-hz-discard-owner-{theme}-{state}.png')
        detail.paste(im.crop((360,390,720,510)).resize((1440,480),Image.Resampling.NEAREST),(0,480*col))
    detail.save(target/f'focus-detail-{theme}.png')
(target/'image-differences.txt').write_text('\n'.join(lines)+'\n')
print('\n'.join(lines))
