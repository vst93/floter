from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import shutil

dest = Path('docs/rounds/P13-jr')
scenarios = ['removed', 'invalid', 'identity', 'repeat', 'competing', 'binding', 'repository', 'command-failure', 'command-recovery']
lines=[]
for theme in ['light','dark']:
    tiles=[]
    for scenario in scenarios:
        before=Path(f'/tmp/floter-P13-jr-{scenario}-{theme}-before.png')
        after=Path(f'/tmp/floter-P13-jr-{scenario}-{theme}-after.png')
        if not before.exists() or not after.exists():
            lines.append(f'MISSING {scenario} {theme}')
            continue
        for src in [before,after]: shutil.copy(src,dest/src.name.removeprefix('floter-P13-jr-'))
        a,b=Image.open(before).convert('RGB'),Image.open(after).convert('RGB')
        diff=ImageChops.difference(a,b)
        box=diff.getbbox()
        pixels=sum(1 for px in diff.getdata() if px != (0,0,0))
        lines.append(f'{scenario} {theme}: changed={pixels}, bounds={box}')
        if box:
            left,top,right,bottom=box
            box=(max(0,left-24), max(0,top-28), min(a.width,right+24), min(a.height,bottom+28))
        else: box=(0,0,a.width,a.height)
        if scenario == 'binding': box=(560,0,960,160)
        if scenario == 'repeat': box=(730,335,960,460)
        if scenario == 'competing': box=(620,680,960,780)
        # Preserve native pixels in each detailed comparison; only the overview scales.
        ca,cb=a.crop(box),b.crop(box)
        col=max(ca.width,300)
        panel=Image.new('RGB',(col*2+20,ca.height+32),'#c9c9cc')
        draw=ImageDraw.Draw(panel)
        draw.text((8,9),f'{scenario} / {theme} / BEFORE',fill='black')
        draw.text((col+28,9),'AFTER',fill='black')
        panel.paste(ca,(0,32));panel.paste(cb,(col+20,32))
        panel.save(dest/f'{scenario}-{theme}-compare.png')
        if scenario in ['removed','repeat','binding','repository','command-failure','command-recovery']:
            panel.thumbnail((1100,430))
            tiles.append(panel)
    canvas=Image.new('RGB',(1100,sum(im.height+14 for im in tiles)),'#bcbcc0')
    y=0
    for im in tiles: canvas.paste(im,(0,y));y+=im.height+14
    canvas.save(dest/f'overview-{theme}.png')
(dest/'image-differences.txt').write_text('\n'.join(lines)+'\n')
print('\n'.join(lines))
