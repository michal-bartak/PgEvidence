#!/usr/bin/env python3
"""Generate build/appicon.png (1024x1024) — the single source Wails uses to
derive every platform icon. Mirrors build/appicon.svg: a PostgreSQL elephant on
an audit document with a green verification seal, on the app's blue tile.

Drawn at 4x supersample and downscaled with LANCZOS for clean anti-aliased edges.
Requires Pillow (no SVG rasterizer needed)."""

import os
from PIL import Image, ImageDraw

SIZE = 1024
SS = 4                      # supersample factor
W = SIZE * SS               # working canvas size
OUT = os.path.join(os.path.dirname(__file__), "..", "build", "appicon.png")

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

    out = Image.alpha_composite(Image.new("RGBA", (W, W), (0, 0, 0, 0)), img)
    out = out.resize((SIZE, SIZE), Image.LANCZOS)
    out.save(os.path.normpath(OUT))
    print("wrote", os.path.normpath(OUT))


if __name__ == "__main__":
    main()
