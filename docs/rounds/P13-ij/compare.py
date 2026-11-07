from pathlib import Path
from PIL import Image, ImageChops, ImageDraw
import shutil

out = Path(__file__).parent
scenes = ["withdraw-command", "withdraw-plan", "withdraw-script", "withdraw-export", "connect-withdraw", "register-withdraw", "withdraw-choose", "register-restore-navigation"]
records = []
for theme in ("light", "dark"):
    board = Image.new("RGB", (1460, len(scenes) * 205), "#ededed" if theme == "light" else "#202020")
    draw = ImageDraw.Draw(board)
    for index, scene in enumerate(scenes):
        paths = [Path(f"/tmp/floter-P13-ij-{scene}-{theme}-{tag}.png") for tag in ("before", "after")]
        before, after = [Image.open(p).convert("RGB") for p in paths]
        assert before.size == after.size
        diff = ImageChops.difference(before, after)
        bbox = diff.getbbox()
        assert bbox, f"No pixel change: {scene} {theme}"
        changed = sum(pixel != (0, 0, 0) for pixel in diff.getdata())
        records.append(f"{scene} {theme}: {changed} changed pixels; bbox={bbox}; size={before.size}")
        top = max(0, (bbox[1] + bbox[3]) // 2 - 85)
        left = max(0, before.width - 720)
        crop = (left, top, left + 720, min(before.height, top + 170))
        draw.text((10, index * 205 + 5), f"{index + 1}. {scene} | BEFORE / AFTER (1:1 crop)", fill="#202020" if theme == "light" else "#eeeeee")
        for x, img in ((10, before), (740, after)):
            board.paste(img.crop(crop), (x, index * 205 + 28))
        for path in paths:
            shutil.copy2(path, out / path.name)
    board.save(out / f"comparison-{theme}.png")
(out / "image-differences.txt").write_text("\n".join(records) + "\n")
print("\n".join(records))
