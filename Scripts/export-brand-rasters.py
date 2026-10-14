#!/usr/bin/env python3
"""Remove the approved artwork's near-white matte without redrawing it."""

from collections import deque
from pathlib import Path

from PIL import Image, ImageChops, ImageFilter


BRAND = Path(__file__).resolve().parents[1] / "Resources" / "Brand"


def transparent_artwork(original: Image.Image) -> Image.Image:
    rgb = original.convert("RGB")
    width, height = rgb.size
    pixels = rgb.load()
    minimum = ImageChops.darker(ImageChops.darker(*rgb.split()[:2]), rgb.split()[2])
    core = minimum.point(lambda value: 255 if value < 220 else 0)
    core = core.filter(ImageFilter.MinFilter(3))
    boundary = ImageChops.subtract(core, core.filter(ImageFilter.MinFilter(3)))
    output = rgb.convert("RGBA")
    output.putalpha(core)
    target = output.load()
    queue = deque((x, y, x, y, 0) for y in range(height) for x in range(width) if boundary.getpixel((x, y)))
    seen = set()
    while queue:
        x, y, fx, fy, distance = queue.popleft()
        if distance == 6:
            continue
        for nx, ny in ((x - 1, y), (x + 1, y), (x, y - 1), (x, y + 1)):
            if not (0 <= nx < width and 0 <= ny < height) or (nx, ny) in seen or core.getpixel((nx, ny)):
                continue
            seen.add((nx, ny))
            queue.append((nx, ny, fx, fy, distance + 1))
            observed, foreground = pixels[nx, ny], pixels[fx, fy]
            contrast = [253 - value for value in foreground]
            alpha = sum((253 - value) * weight for value, weight in zip(observed, contrast)) / sum(weight * weight for weight in contrast)
            alpha = min(1.0, max(0.0, alpha))
            if alpha < 0.02:
                continue
            color = tuple(round(min(255, max(0, (value - 253 * (1 - alpha)) / alpha))) for value in observed)
            target[nx, ny] = (*color, round(alpha * 255))
    return output


def main() -> None:
    original = Image.open(BRAND / "quosparr-logo-approved.png")
    source = transparent_artwork(original)
    source.save(BRAND / "quosparr-logo-source.png")
    for size in (64, 128, 256, 512, 1024, 2048, 4096):
        folder = BRAND if size in (1024, 4096) else BRAND / "png"
        source.resize((size, size), Image.Resampling.LANCZOS).save(folder / f"quosparr-logo-{size}.png")
    print(f"Exported transparent master from approved {original.width} x {original.height} artwork and 7 PNG sizes")


if __name__ == "__main__":
    main()
