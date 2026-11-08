"""Build labeled comparisons from ui.Tester PNGs; never redraw product UI."""
from pathlib import Path
from shutil import copyfile
from PIL import Image, ImageChops, ImageDraw

out = Path(__file__).resolve().parent
names = ["name", "program", "args", "versionargs", "description", "mode", "choose", "language"]
lines = []
for theme in ["light", "dark"]:
    overview = Image.new("RGB", (1440, 1120), "#eeeeee")
    for i, name in enumerate(names):
        frames = []
        for stage in ["before", "after"]:
            filename = f"floter-P13-kj-{name}-{theme}-{stage}.png"
            copyfile(Path("/tmp") / filename, out / filename)
            frames.append(Image.open(out / filename).convert("RGB"))
        before, after = frames
        assert before.size == after.size == (720, 520)
        changed = sum(pixel != (0, 0, 0) for pixel in ImageChops.difference(before, after).get_flattened_data())
        assert changed > 0, (name, theme)
        lines.append(f"{name} {theme}: {changed} changed pixels")
        comparison = Image.new("RGB", (1440, 548), "#eeeeee")
        draw = ImageDraw.Draw(comparison)
        draw.text((12, 7), f"{name} / {theme} / before", fill="black")
        draw.text((732, 7), f"{name} / {theme} / after", fill="black")
        comparison.paste(before, (0, 28))
        comparison.paste(after, (720, 28))
        comparison.save(out / f"{name}-{theme}-comparison.png")
        overview.paste(comparison.resize((720, 274)), ((i % 2) * 720, (i // 2) * 280))
        if name in ["name", "choose", "language"]:
            detail = Image.new("RGB", (2400, 368), "#eeeeee")
            for j, frame in enumerate(frames):
                detail.paste(frame.crop((408, 36, 708, 128)).resize((1200, 368), Image.Resampling.NEAREST), (j * 1200, 0))
            detail.save(out / f"{name}-{theme}-detail4x.png")
    overview.save(out / f"overview-{theme}.png")
(out / "image-differences.txt").write_text("\n".join(lines) + "\n")
print("\n".join(lines))
