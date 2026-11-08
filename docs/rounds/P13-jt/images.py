from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import shutil

out=Path('/home/tar/workspace/floter/docs/rounds/P13-jt')
scenarios=['repeat','competing','manifest','descriptor','publisher','runtime','repository-start','repository-completion']
counts=[]
for theme in ['light','dark']:
    tiles=[]
    for i,scenario in enumerate(scenarios,1):
        pair=[]
        for tag in ['before','after']:
            src=Path(f'/tmp/floter-P13-jt-{scenario}-{theme}-{tag}.png')
            shutil.copy2(src,out/src.name.removeprefix('floter-P13-jt-'))
            pair.append(Image.open(src).convert('RGB'))
        box=(195,990,940,1520) if not scenario.startswith('repository') else (195,0,940,195)
        crop=[p.crop(box) for p in pair]
        diff=ImageChops.difference(*crop)
        changed=sum(pixel != (0,0,0) for pixel in diff.getdata())
        counts.append(f'{scenario}/{theme}: {changed} changed pixels; crop={box}')
        assert changed > 0, (scenario,theme)
        canvas=Image.new('RGB',(crop[0].width*2,crop[0].height+28),'#eee')
        draw=ImageDraw.Draw(canvas)
        draw.text((12,7),f'{i}. {scenario} | {theme} | BEFORE',fill='black')
        draw.text((crop[0].width+12,7),'AFTER',fill='black')
        for x,p in enumerate(crop): canvas.paste(p,(x*p.width,28))
        canvas.save(out/f'{scenario}-{theme}-compare.png')
        canvas.thumbnail((960,370))
        tiles.append(canvas)
    board=Image.new('RGB',(960,sum(t.height+12 for t in tiles)),'#ddd')
    y=0
    for tile in tiles: board.paste(tile,(0,y)); y+=tile.height+12
    board.save(out/f'overview-{theme}.png')
    for scenario, box in [('repeat',(540,1450,922,1495)),('repository-start',(520,10,930,130))]:
        src=Image.open(out/f'{scenario}-{theme}-compare.png')
        # Original full-resolution frame, nearest-neighbour magnification.
        src=Image.open(out/f'{scenario}-{theme}-after.png').crop(box)
        src.resize((src.width*4,src.height*4),Image.Resampling.NEAREST).save(out/f'{scenario}-{theme}-detail.png')
(out/'image-differences.txt').write_text('\n'.join(counts)+'\n')
print('\n'.join(counts))
