#!/usr/bin/env python3
"""Generate the app icon in every form the build needs, from one drawing:

  build/appicon.png    1024x1024 — Wails' icon source (and docs favicon)
  build/appicon.icns   full macOS iconset (16..512 at 1x AND 2x)
  build/windows/icon.ico  multi-size Windows icon

Why we emit .icns/.ico ourselves (see CLAUDE.md decision log):
- The .icns Wails generates omits the @1x sizes, so macOS Finder/Dock/cmd-tab
  fall back to a generic icon. `make dist` copies our complete .icns into the
  bundle and re-signs.
- Wails never regenerates build/windows/icon.ico from appicon.png at all.

Drawn at 4x supersample and downscaled with LANCZOS for clean anti-aliased edges.
Requires Pillow (no SVG rasterizer needed); .icns also needs macOS `iconutil`."""

import os
import shutil
import subprocess
import tempfile
from PIL import Image, ImageDraw

SIZE = 1024
SS = 4                      # supersample factor
W = SIZE * SS               # working canvas size
BUILD = os.path.normpath(os.path.join(os.path.dirname(__file__), "..", "build"))
OUT = os.path.join(BUILD, "appicon.png")
ICNS_OUT = os.path.join(BUILD, "appicon.icns")
ICO_OUT = os.path.join(BUILD, "windows", "icon.ico")

BLUE_TOP = (47, 111, 208)   # #2f6fd0
BLUE_BOT = (90, 168, 255)   # #5aa8ff
PG = (51, 103, 145)         # #336791 Postgres elephant blue
PUPIL = (32, 56, 79)        # #20384f
GREEN = (43, 178, 76)       # #2bb24c
WHITE = (255, 255, 255, 255)


def s(v):
    return int(round(v * SS))


def lerp(a, b, t):
    return tuple(int(round(a[i] + (b[i] - a[i]) * t)) for i in range(3))


def bezier(p0, p1, p2, p3, n=80):
    pts = []
    for i in range(n + 1):
        t = i / n
        u = 1 - t
        x = u**3 * p0[0] + 3 * u**2 * t * p1[0] + 3 * u * t**2 * p2[0] + t**3 * p3[0]
        y = u**3 * p0[1] + 3 * u**2 * t * p1[1] + 3 * u * t**2 * p2[1] + t**3 * p3[1]
        pts.append((s(x), s(y)))
    return pts


def thick_path(draw, pts, width, color):
    """Polyline with rounded joints and round end caps."""
    draw.line(pts, fill=color, width=width, joint="curve")
    r = width / 2
    for x, y in (pts[0], pts[-1]):
        draw.ellipse([x - r, y - r, x + r, y + r], fill=color)


def main():
    # Blue tile gradient (vertical), masked to a rounded square.
    tile = Image.new("RGB", (W, W))
    td = ImageDraw.Draw(tile)
    for y in range(W):
        td.line([(0, y), (W, y)], fill=lerp(BLUE_TOP, BLUE_BOT, y / (W - 1)))
    mask = Image.new("L", (W, W), 0)
    ImageDraw.Draw(mask).rounded_rectangle([0, 0, W - 1, W - 1], radius=s(230), fill=255)

    img = Image.new("RGBA", (W, W), (0, 0, 0, 0))
    tile_rgba = tile.convert("RGBA")
    tile_rgba.putalpha(mask)
    img.alpha_composite(tile_rgba)

    d = ImageDraw.Draw(img)

    # Document.
    d.rounded_rectangle([s(288), s(196), s(736), s(828)], radius=s(46), fill=WHITE)

    def ellipse(cx, cy, rx, ry, fill):
        d.ellipse([s(cx - rx), s(cy - ry), s(cx + rx), s(cy + ry)], fill=fill)

    # Elephant: ears, head, trunk, eyes.
    ellipse(412, 398, 96, 128, PG)
    ellipse(612, 398, 96, 128, PG)
    ellipse(512, 432, 134, 152, PG)
    thick_path(d, bezier((512, 548), (480, 628), (486, 706), (558, 708)), s(74), PG)
    ellipse(470, 414, 23, 23, WHITE)
    ellipse(554, 414, 23, 23, WHITE)
    ellipse(470, 414, 11, 11, PUPIL)
    ellipse(554, 414, 11, 11, PUPIL)

    # Green verification seal with white outline + check mark.
    d.ellipse([s(706 - 132), s(730 - 132), s(706 + 132), s(730 + 132)],
              fill=GREEN, outline=WHITE, width=s(26))
    thick_path(d, [(s(634), s(732)), (s(690), s(788)), (s(784), s(666))], s(34), WHITE)

    icon = Image.alpha_composite(Image.new("RGBA", (W, W), (0, 0, 0, 0)), img)
    icon = icon.resize((SIZE, SIZE), Image.LANCZOS)
    icon.save(OUT)
    print("wrote", OUT)

    write_icns(icon)
    write_ico(icon)


def write_ico(icon):
    sizes = [(256, 256), (128, 128), (64, 64), (48, 48), (32, 32), (16, 16)]
    os.makedirs(os.path.dirname(ICO_OUT), exist_ok=True)
    icon.save(ICO_OUT, format="ICO", sizes=sizes)
    print("wrote", ICO_OUT)


def write_icns(icon):
    # A complete iconset has both @1x and @2x for 16/32/128/256/512.
    if not shutil.which("iconutil"):
        print("skip", ICNS_OUT, "(iconutil not found — macOS only)")
        return
    specs = [
        ("icon_16x16.png", 16), ("icon_16x16@2x.png", 32),
        ("icon_32x32.png", 32), ("icon_32x32@2x.png", 64),
        ("icon_128x128.png", 128), ("icon_128x128@2x.png", 256),
        ("icon_256x256.png", 256), ("icon_256x256@2x.png", 512),
        ("icon_512x512.png", 512), ("icon_512x512@2x.png", 1024),
    ]
    with tempfile.TemporaryDirectory() as tmp:
        iconset = os.path.join(tmp, "icon.iconset")
        os.makedirs(iconset)
        for name, px in specs:
            icon.resize((px, px), Image.LANCZOS).save(os.path.join(iconset, name))
        subprocess.run(["iconutil", "-c", "icns", iconset, "-o", ICNS_OUT], check=True)
    print("wrote", ICNS_OUT)


if __name__ == "__main__":
    main()
