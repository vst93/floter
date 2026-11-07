"""Archive real ui.Tester frames and make side-by-side crops; no invented UI."""
from pathlib import Path
import shutil
from PIL import Image, ImageChops, ImageDraw

dest = Path(__file__).resolve().parent
prefix = "floter-P13-jo-"
main = ["diagnosis-failed", "clipboard-failed", "reconnect-failed", "repair-failed",
        "install-failed", "remove-failed", "health-owner-manifest", "health-recovery-enabled"]
rows = []
for before_path in sorted(Path("/tmp").glob(prefix + "*-before.png")):
    stem = before_path.name[len(prefix):-len("-before.png")]
    after_path = before_path.with_name(prefix + stem + "-after.png")
    if not after_path.exists():
        continue
    before, after = Image.open(before_path).convert("RGB"), Image.open(after_path).convert("RGB")
    assert before.size == after.size, (stem, before.size, after.size)
    for path in [before_path, after_path]:
        shutil.copy2(path, dest / path.name[len(prefix):])
    delta = ImageChops.difference(before, after)
    pixels = sum(pixel != (0, 0, 0) for pixel in delta.get_flattened_data())
    rows.append(f"{stem}: {before.width}x{before.height}; changed_pixels={pixels}")
    scenario = stem.rsplit("-", 1)[0]
    if scenario not in main:
        continue
    crop = (200, 990, 940, 1450) if scenario.startswith("health-") else (610, 48, 950, 114)
    b, a = before.crop(crop), after.crop(crop)
    board = Image.new("RGB", (b.width * 2 + 12, b.height + 26), "#dedede")
    draw = ImageDraw.Draw(board)
    draw.text((8, 5), "BEFORE", fill="black")
    draw.text((b.width + 20, 5), "AFTER", fill="black")
    board.paste(b, (0, 26))
    board.paste(a, (b.width + 12, 26))
    board.save(dest / (stem + "-comparison.png"))
(dest / "image-differences.txt").write_text("\n".join(rows) + "\n")
print(f"Archived {len(rows) * 2} original frames; {len(main) * 2} comparison crops")
for theme in ["dark", "light"]:
    crops = [Image.open(dest / (name + "-" + theme + "-comparison.png")) for name in main[:6]]
    overview = Image.new("RGB", (max(x.width for x in crops), sum(x.height for x in crops)), "white")
    y = 0
    for crop in crops:
        overview.paste(crop, (0, y))
        y += crop.height
    overview.save(dest / ("feedback-" + theme + "-overview.png"))
