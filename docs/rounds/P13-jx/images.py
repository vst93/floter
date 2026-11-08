from pathlib import Path
import shutil
from PIL import Image, ImageChops, ImageDraw

out = Path(__file__).parent
scenarios = ["registry-download", "registry-cancel", "registry-busy", "registry-blank",
             "registry-commit", "registry-keyboard", "folder-keyboard", "folder-error"]
rows = []
for theme in ("light", "dark"):
    tiles = []
    for scenario in scenarios:
        pair = []
        for stage in ("before", "after"):
            src = Path(f"/tmp/floter-P13-jx-{scenario}-{theme}-{stage}.png")
            dest = out / f"{scenario}-{theme}-{stage}.png"
            shutil.copyfile(src, dest)
            pair.append(Image.open(dest).convert("RGB"))
        diff = ImageChops.difference(*pair)
        box = diff.getbbox()
        changed = sum(pixel != (0, 0, 0) for pixel in diff.getdata())
        rows.append(f"{scenario} {theme}: {changed} changed pixels; bbox={box}")
        canvas = Image.new("RGB", (pair[0].width * 2, pair[0].height + 28), "#cccccc")
        draw = ImageDraw.Draw(canvas)
        for i, stage in enumerate(("before", "after")):
            x = i * pair[0].width
            draw.text((x + 12, 8), f"{scenario} / {theme} / {stage}", fill="black")
            canvas.paste(pair[i], (x, 28))
        canvas.save(out / f"{scenario}-{theme}-comparison.png")
        # 1x details are crops of actual frames; no UI is drawn by this script.
        if box:
            x0, y0, x1, y1 = box
            box = (max(0, x0-12), max(0, y0-12), min(pair[0].width, x1+12), min(pair[0].height, y1+12))
            detail = Image.new("RGB", ((box[2]-box[0])*2, box[3]-box[1]), "#cccccc")
            for i, im in enumerate(pair):
                detail.paste(im.crop(box), (i*(box[2]-box[0]), 0))
            detail.save(out / f"{scenario}-{theme}-detail.png")
            if scenario in ("registry-busy", "registry-keyboard", "folder-keyboard"):
                detail.resize((detail.width*4, detail.height*4), Image.Resampling.NEAREST).save(out / f"{scenario}-{theme}-detail4x.png")
            detail.thumbnail((960, 320))
            tile = Image.new("RGB", (960, detail.height+24), "#cccccc")
            ImageDraw.Draw(tile).text((8, 6), f"{scenario}: before | after", fill="black")
            tile.paste(detail, (0,24))
            tiles.append(tile)
    overview = Image.new("RGB", (960, sum(tile.height for tile in tiles)), "#cccccc")
    y = 0
    for tile in tiles:
        overview.paste(tile, (0,y))
        y += tile.height
    overview.save(out / f"overview-{theme}.png")
(out / "image-differences.txt").write_text("\n".join(rows)+"\n")
