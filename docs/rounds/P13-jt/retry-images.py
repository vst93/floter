"""Pair recovered original frames with this continuation's real Tester frames."""
from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import shutil

out = Path(__file__).resolve().parent
scenarios = ["repeat", "competing", "manifest", "descriptor", "publisher", "runtime",
             "repository-start", "repository-completion"]
measurements = []
for theme in ["light", "dark"]:
    tiles = []
    for index, scenario in enumerate(scenarios, 1):
        source = Path(f"/tmp/floter-P13-jt-{scenario}-{theme}-retry.png")
        target = out / f"{scenario}-{theme}-retry.png"
        shutil.copy2(source, target)
        before = Image.open(out / f"{scenario}-{theme}-before.png").convert("RGB")
        after = Image.open(target).convert("RGB")
        assert before.size == after.size == (960, 1600)
        crop = (195, 990, 940, 1520) if not scenario.startswith("repository") else (195, 0, 940, 195)
        pair = [before.crop(crop), after.crop(crop)]
        changed = sum(pixel != (0, 0, 0) for pixel in ImageChops.difference(*pair).getdata())
        assert changed > 0
        measurements.append(f"{scenario}/{theme}: {changed} changed pixels; crop={crop}")
        width, height = pair[0].size
        canvas = Image.new("RGB", (width * 2, height + 28), "#eee")
        draw = ImageDraw.Draw(canvas)
        draw.text((12, 7), f"{index}. {scenario} | {theme} | ORIGINAL BEFORE", fill="black")
        draw.text((width + 12, 7), "CONTINUATION AFTER", fill="black")
        for column, frame in enumerate(pair):
            canvas.paste(frame, (column * width, 28))
        canvas.save(out / f"{scenario}-{theme}-retry-compare.png")
        canvas.thumbnail((960, 370))
        tiles.append(canvas)
    board = Image.new("RGB", (960, sum(tile.height + 12 for tile in tiles)), "#ddd")
    y = 0
    for tile in tiles:
        board.paste(tile, (0, y))
        y += tile.height + 12
    board.save(out / f"overview-{theme}-retry.png")
(out / "retry-image-differences.txt").write_text("\n".join(measurements) + "\n")
print("Exported 16 current frames, 16 comparisons and two overview boards.")
