"""Archive existing ui.Tester frames; compose, never re-render a state."""
from pathlib import Path
from shutil import copy2
from PIL import Image, ImageDraw

out = Path(__file__).parent / "images"
out.mkdir(exist_ok=True)
for source in Path('/tmp').glob('floter-P13-kt-*.png'):
    copy2(source, out / source.name)
for theme in ('dark', 'light'):
    sheet = Image.new('RGB', (1440, 1080), '#303033')
    draw = ImageDraw.Draw(sheet)
    for row, case in enumerate(('health-text', 'result-check')):
        for col, state in enumerate(('before', 'after')):
            name = f'floter-P13-kt-{case}-{theme}-{state}.png'
            frame = Image.open(out / name).convert('RGB')
            sheet.paste(frame, (720 * col, 540 * row + 20))
            draw.text((720 * col + 10, 540 * row + 4), f'{case} / {theme} / {state}', fill='white')
    sheet.save(out / f'comparison-{theme}.png')
print(f'Archived {len(list(out.glob("floter-*.png")))} frames and 2 comparisons')
