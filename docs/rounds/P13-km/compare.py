from pathlib import Path
from PIL import Image, ImageDraw
import shutil

dest = Path('/home/tar/workspace/floter/docs/rounds/P13-km')
root = Path('/tmp')
pairs = []
for before in sorted(root.glob('floter-P13-km-*-before.png')):
    after = before.with_name(before.name.replace('-before.png', '-after.png'))
    if not after.exists():
        continue
    a, b = Image.open(before).convert('RGB'), Image.open(after).convert('RGB')
    rest = '-rest-' in before.name
    if rest:
        assert a.tobytes() == b.tobytes(), f'rest changed: {before.name}'
    canvas = Image.new('RGB', (a.width+b.width, a.height+28), '#e9e9e9')
    canvas.paste(a, (0,28)); canvas.paste(b, (a.width,28))
    draw = ImageDraw.Draw(canvas)
    draw.text((12,8), 'BEFORE', fill='black'); draw.text((a.width+12,8), 'AFTER', fill='black')
    name = before.name.removeprefix('floter-P13-km-').replace('-before.png','-compare.png')
    canvas.save(dest/name)
    if not rest:
        pairs.append(name)
    shutil.copy2(before,dest/before.name); shutil.copy2(after,dest/after.name)
for dark in ['dark','light']:
    names = [f'text-top-{dark}-compare.png',f'select-bottom-{dark}-compare.png',f'multi-page-top-{dark}-compare.png',f'multi-option-{dark}-compare.png',f'multi-moved-{dark}-compare.png']
    imgs=[]
    for name in names:
        im=Image.open(dest/name)
        im.thumbnail((1000,380))
        imgs.append(im.copy())
    board=Image.new('RGB',(1000,sum(im.height+24 for im in imgs)), '#ededed')
    y=0; draw=ImageDraw.Draw(board)
    for name,im in zip(names,imgs):
        draw.text((8,y+4),name,fill='black'); board.paste(im,(0,y+24)); y+=im.height+24
    board.save(dest/f'overview-{dark}.png')
print(f'{len(pairs)} state comparisons; all rest pairs byte-identical')
