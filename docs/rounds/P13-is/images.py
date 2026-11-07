"""Assemble actual ui.Tester frames; never redraw product screenshots."""
from pathlib import Path
from PIL import Image, ImageChops, ImageDraw

root = Path(__file__).parent
rows = []
counts = []
for before in sorted(root.glob("floter-P13-is-*-before.png")):
    after = before.with_name(before.name.replace("-before", "-after"))
    if not after.exists():
        continue
    left, right = Image.open(before).convert("RGB"), Image.open(after).convert("RGB")
    diff = ImageChops.difference(left, right)
    count = sum(p != (0, 0, 0) for p in diff.getdata())
    counts.append(f"{before.name}: {count} changed pixels")
    board = Image.new("RGB", (left.width * 2, left.height + 28), "#dddddd")
    draw = ImageDraw.Draw(board)
    draw.text((10, 8), before.stem, fill="black")
    draw.text((left.width + 10, 8), "after", fill="black")
    board.paste(left, (0, 28)); board.paste(right, (left.width, 28))
    board.save(before.with_name(before.name.replace("-before", "-comparison")))
    board.thumbnail((720, 364))
    rows.append(board)
overview = Image.new("RGB", (1440, sum(max(a.height, b.height) for a, b in zip(rows[::2], rows[1::2]))), "#dddddd")
y = 0
for a, b in zip(rows[::2], rows[1::2]):
    overview.paste(a, (0, y)); overview.paste(b, (720, y)); y += max(a.height, b.height)
overview.save(root / "overview.png")
(root / "image-differences.txt").write_text("\n".join(counts) + "\n")
