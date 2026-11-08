from pathlib import Path
from shutil import copyfile
from PIL import Image, ImageChops, ImageDraw

root = Path(__file__).parent
names = ['registry-name', 'registry-version', 'text', 'password', 'path', 'number', 'select', 'boolean']
lines = []
for mode in ['light', 'dark']:
    overview = Image.new('RGB', (1440, 4*274), '#777777')
    for i, name in enumerate(names):
        pair = []
        for tag in ['before', 'after']:
            path = Path(f'/tmp/floter-P13-kl-{name}-{mode}-{tag}.png')
            copyfile(path, root / f'{name}-{mode}-{tag}.png')
            pair.append(Image.open(path).convert('RGB'))
        diff = ImageChops.difference(*pair)
        changed = sum(p != (0, 0, 0) for p in diff.getdata())
        assert changed > 0, (name, mode, 'no change')
        lines.append(f'{name} {mode}: changed={changed}')
        comparison = Image.new('RGB', (1440, 548), '#ffffff')
        draw = ImageDraw.Draw(comparison)
        draw.text((12, 8), f'{name} {mode} BEFORE', fill='#111111')
        draw.text((732, 8), 'AFTER', fill='#111111')
        comparison.paste(pair[0], (0, 28))
        comparison.paste(pair[1], (720, 28))
        comparison.save(root / f'{name}-{mode}-comparison.png')
        overview.paste(comparison.resize((720, 274)), ((i%2)*720, (i//2)*274))
        if name in ['registry-version', 'text', 'boolean']:
            detail = Image.new('RGB', (4128, 400), '#ffffff')
            for j, original in enumerate(pair):
                crop = original.crop((185, 45, 701, 145)).resize((2064, 400), Image.Resampling.NEAREST)
                detail.paste(crop, (j*2064, 0))
            detail.save(root / f'{name}-{mode}-detail.png')
        before = Path(f'/tmp/floter-P13-kl-{name}-rest-{mode}-before.png')
        after = Path(f'/tmp/floter-P13-kl-{name}-rest-{mode}-after.png')
        assert before.read_bytes() == after.read_bytes(), (name, mode, 'rest changed')
        lines.append(f'{name} {mode}: resting PNG byte-identical')
        copyfile(after, root / f'{name}-{mode}-rest.png')
    overview.save(root / f'overview-{mode}.png')
(root / 'image-differences.txt').write_text('\n'.join(lines)+'\n')
