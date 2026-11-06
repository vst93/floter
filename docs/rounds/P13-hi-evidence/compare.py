from pathlib import Path
from PIL import Image, ImageChops, ImageDraw

root = Path(__file__).parent
names = ["config-type-draft", "config-type-undo", "number-bounds", "text-bounds",
         "plugin-text-type", "plugin-number-type", "removed-multi", "removed-plugin-text"]
lines = []
for theme in ("light", "dark"):
    board = Image.new("RGB", (1440, 8 * 250), "#eeeeee" if theme == "light" else "#161616")
    draw = ImageDraw.Draw(board)
    for row, name in enumerate(names):
        before = Image.open(root / f"floter-P13-hi-{name}-{theme}-before.png").convert("RGB")
        after = Image.open(root / f"floter-P13-hi-{name}-{theme}-after.png").convert("RGB")
        diff = ImageChops.difference(before, after)
        box = diff.getbbox()
        assert box is not None, f"No visible change: {name}/{theme}"
        lines.append(f"{name}/{theme}: difference bbox {box}")
        # UI occupies the top of these real native frames; no rescaling.
        board.paste(before.crop((0, 0, 720, 225)), (0, row * 250 + 25))
        board.paste(after.crop((0, 0, 720, 225)), (720, row * 250 + 25))
        draw.text((10, row * 250 + 6), name + " / BEFORE", fill="#888888")
        draw.text((730, row * 250 + 6), name + " / AFTER", fill="#888888")
    board.save(root / f"comparison-{theme}.png")
    for name, box in (("number-bounds", (145, 58, 350, 112)),
                      ("plugin-number-type", (610, 44, 699, 83))):
        before = Image.open(root / f"floter-P13-hi-{name}-{theme}-before.png")
        after = Image.open(root / f"floter-P13-hi-{name}-{theme}-after.png")
        w, h = box[2] - box[0], box[3] - box[1]
        detail = Image.new("RGB", (w * 8, h * 4))
        detail.paste(before.crop(box).resize((w * 4, h * 4), Image.Resampling.NEAREST), (0, 0))
        detail.paste(after.crop(box).resize((w * 4, h * 4), Image.Resampling.NEAREST), (w * 4, 0))
        detail.save(root / f"detail-{name}-{theme}.png")
(root / "image-differences.txt").write_text("\n".join(lines) + "\n")
