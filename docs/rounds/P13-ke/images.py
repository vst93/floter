from pathlib import Path
import json
import shutil
from PIL import Image, ImageChops, ImageDraw

out = Path(__file__).parent
names = ["field-removed", "field-kind", "submit-unavailable", "form-cancel",
         "form-closed", "form-empty", "config-cancel", "config-discard"]
differences = {}
for theme in ["light", "dark"]:
    overview = Image.new("RGB", (1440, 1160), "#eeeeee")
    for i, name in enumerate(names):
        frames = []
        for phase in ["before", "after"]:
            src = Path(f"/tmp/floter-P13-ke-{name}-{theme}-{phase}.png")
            shutil.copyfile(src, out / f"{name}-{theme}-{phase}.png")
            frames.append(Image.open(src).convert("RGB"))
        delta = ImageChops.difference(*frames)
        changed = sum(1 for pixel in delta.getdata() if pixel != (0, 0, 0))
        if not changed:
            raise RuntimeError(f"no visual change: {name}-{theme}")
        differences[f"{name}-{theme}"] = changed
        pair = Image.new("RGB", (1440, 550), "#eeeeee")
        draw = ImageDraw.Draw(pair)
        draw.text((12, 8), f"{name} / {theme} / BEFORE", fill="black")
        draw.text((732, 8), "AFTER", fill="black")
        for j, frame in enumerate(frames):
            pair.paste(frame, (j * 720, 30))
        pair.save(out / f"{name}-{theme}-comparison.png")
        overview.paste(pair.resize((720, 275)), ((i % 2) * 720, (i // 2) * 290))
    overview.save(out / f"overview-{theme}.png")
    detail = Image.open(out / f"field-kind-{theme}-after.png")
    detail.crop((455, 48, 700, 90)).resize((980, 168), Image.Resampling.NEAREST).save(out / f"focus-{theme}-4x.png")
(out / "image-differences.json").write_text(json.dumps(differences, indent=2) + "\n")
print(json.dumps(differences))
