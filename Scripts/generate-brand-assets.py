#!/usr/bin/env python3
"""Wrap the approved raster artwork in SVG and add the outlined wordmark."""

from pathlib import Path
import base64

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont


ROOT = Path(__file__).resolve().parents[1]
BRAND = ROOT / "Resources" / "Brand"
FONT = Path("/System/Library/Fonts/Supplemental/Arial Rounded Bold.ttf")
WORD = "Quosparr"


def outlined_wordmark() -> tuple[str, float]:
    font = TTFont(FONT)
    glyphs = font.getGlyphSet()
    cmap = font.getBestCmap()
    scale = 0.082
    cursor = 236.0
    paths: list[str] = []
    for character in WORD:
        glyph_name = cmap[ord(character)]
        pen = SVGPathPen(glyphs)
        transformed = TransformPen(pen, (scale, 0, 0, -scale, cursor, 172))
        glyphs[glyph_name].draw(transformed)
        paths.append(f'    <path d="{pen.getCommands()}"/>')
        cursor += (glyphs[glyph_name].width - 24) * scale
    return "\n".join(paths), cursor


def raster_symbol() -> str:
    source = (BRAND / "quosparr-logo-source.png").read_bytes()
    encoded = base64.b64encode(source).decode("ascii")
    return f'''  <g id="symbol">
    <image width="256" height="256" href="data:image/png;base64,{encoded}"/>
  </g>'''


def main() -> None:
    symbol = raster_symbol()
    master = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" width="256" height="256" role="img" aria-labelledby="title desc">
  <title id="title">Quosparr sparrow logo</title>
  <desc id="desc">Embedded approved raster artwork preserving the original soft sculpted shading; not vector paths.</desc>
{symbol}
</svg>
'''
    (BRAND / "quosparr-logo.svg").write_text(master, encoding="utf-8")
    wordmark, end_x = outlined_wordmark()
    width = round(end_x + 34)
    horizontal = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} 256" width="{width}" height="256" role="img" aria-labelledby="title desc">
  <title id="title">Quosparr horizontal logo</title>
  <desc id="desc">Quosparr sparrow symbol with an outlined rounded wordmark.</desc>
  <g transform="translate(20 32) scale(0.75)">
{symbol}
  </g>
  <g id="wordmark" fill="#B3582D">
{wordmark}
  </g>
</svg>
'''
    (BRAND / "quosparr-logo-horizontal.svg").write_text(horizontal, encoding="utf-8")
    print(f"wrote Resources/Brand/quosparr-logo-horizontal.svg ({width} x 256)")


if __name__ == "__main__":
    main()
