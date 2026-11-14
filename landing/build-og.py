#!/usr/bin/env python3
"""Generate og-image.svg (self-contained) for link previews.

Run: python3 build-og.py && rsvg-convert og-image.svg -o og-image.png

ponytail: macOS only (system font + Apple Color Emoji, via Pillow). rsvg draws
emoji in monochrome, so they're pre-rendered to PNG and embedded. After
regenerating, bump the ?v= on og:image in the HTML so platforms refetch it.
"""
import base64
import io
import pathlib

from PIL import Image, ImageDraw, ImageFont

HERE = pathlib.Path(__file__).parent

# Brand tokens from styles.css (oklch converted to sRGB hex).
BG, CHIP, BORDER = "#181411", "#2b2520", "#352f2a"
TEXT, MUTED, ACCENT, PLUS = "#eeebe5", "#b7b0a8", "#fabb41", "#58da98"
FONT = "system-ui, -apple-system, Helvetica Neue, Arial, sans-serif"


def data_uri(png: bytes) -> str:
    return "data:image/png;base64," + base64.b64encode(png).decode()


def emoji_uri(ch: str) -> str:
    # Apple Color Emoji only ships bitmap strikes; 160 is the largest.
    font = ImageFont.truetype("/System/Library/Fonts/Apple Color Emoji.ttc", 160)
    img = Image.new("RGBA", (160, 160))
    ImageDraw.Draw(img).text((0, 0), ch, font=font, embedded_color=True)
    buf = io.BytesIO()
    img.save(buf, "PNG")
    return data_uri(buf.getvalue())


icon = data_uri((HERE / "plusplus-icon.png").read_bytes())

# Same reactions and values as the bot's defaults.
pills, x = [], 80
for ch, pts in [("👏", 1), ("🙌", 2), ("⭐", 3), ("🔥", 4), ("🏆", 5)]:
    pills.append(f"""
  <rect x="{x}" y="478" width="118" height="64" rx="32" fill="{CHIP}" stroke="{BORDER}" stroke-width="2"/>
  <image href="{emoji_uri(ch)}" x="{x + 18}" y="492" width="36" height="36"/>
  <text x="{x + 64}" y="522" font-family="{FONT}" font-weight="700" font-size="28" fill="{PLUS}">+{pts}</text>""")
    x += 132

svg = f"""<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <defs>
    <radialGradient id="glow" cx="88%" cy="0%" r="70%">
      <stop offset="0" stop-color="{ACCENT}" stop-opacity="0.16"/>
      <stop offset="0.7" stop-color="{ACCENT}" stop-opacity="0"/>
    </radialGradient>
  </defs>

  <rect width="1200" height="630" fill="{BG}"/>
  <rect width="1200" height="630" fill="url(#glow)"/>
  <rect x="0" y="624" width="1200" height="6" fill="{ACCENT}"/>

  <image href="{icon}" x="44" y="40" width="132" height="92" preserveAspectRatio="xMidYMid meet"/>
  <text x="160" y="99" font-family="{FONT}" font-weight="700" font-size="36" fill="{TEXT}">plusplus</text>
  <text x="1120" y="99" font-family="{FONT}" font-size="26" fill="{MUTED}" text-anchor="end">pluspluskarma.dev</text>

  <text x="76" y="262" font-family="{FONT}" font-weight="800" font-size="88" letter-spacing="-1.5" fill="{TEXT}">Karma for your Slack,</text>
  <text x="76" y="360" font-family="{FONT}" font-weight="800" font-size="88" letter-spacing="-1.5" fill="{ACCENT}">made simple.</text>

  <text x="80" y="432" font-family="{FONT}" font-size="32" fill="{MUTED}">React to a teammate's message to give them karma.</text>
{"".join(pills)}
</svg>
"""

(HERE / "og-image.svg").write_text(svg)
print("wrote og-image.svg")
