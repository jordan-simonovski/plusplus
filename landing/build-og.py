#!/usr/bin/env python3
"""Generate og-image.svg (self-contained) for link previews.

Run: python3 build-og.py && rsvg-convert og-image.svg -o og-image.png
"""
import base64
import pathlib

HERE = pathlib.Path(__file__).parent
icon_b64 = base64.b64encode((HERE / "plusplus-icon.png").read_bytes()).decode()
icon_uri = f"data:image/png;base64,{icon_b64}"

svg = f"""<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="1200" height="630" viewBox="0 0 1200 630">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="#1f2937"/>
      <stop offset="1" stop-color="#111827"/>
    </linearGradient>
    <radialGradient id="glow" cx="80%" cy="-5%" r="65%">
      <stop offset="0" stop-color="#38bdf8" stop-opacity="0.22"/>
      <stop offset="0.65" stop-color="#38bdf8" stop-opacity="0"/>
    </radialGradient>
  </defs>

  <rect width="1200" height="630" fill="url(#bg)"/>
  <rect width="1200" height="630" fill="url(#glow)"/>
  <rect x="0" y="626" width="1200" height="4" fill="#38bdf8"/>

  <image xlink:href="{icon_uri}" x="700" y="150" width="470" height="330" preserveAspectRatio="xMidYMid meet" opacity="0.97"/>

  <image xlink:href="{icon_uri}" x="62" y="58" width="96" height="64" preserveAspectRatio="xMidYMid meet"/>
  <text x="150" y="106" font-family="Arial, Helvetica, sans-serif" font-weight="bold" font-size="36" fill="#e5e9f0">plusplus</text>

  <text x="78" y="288" font-family="Arial, Helvetica, sans-serif" font-weight="bold" font-size="80" letter-spacing="-3" fill="#ffffff">Karma for your Slack,</text>
  <text x="78" y="380" font-family="Arial, Helvetica, sans-serif" font-weight="bold" font-size="80" letter-spacing="-3" fill="#38bdf8">made simple.</text>

  <text x="82" y="446" font-family="Arial, Helvetica, sans-serif" font-size="31" fill="#9aa7bd">Mention a teammate with ++ to award points, -- to dock them.</text>

  <rect x="80" y="500" width="290" height="66" rx="16" fill="#34d399"/>
  <text x="225" y="543" font-family="Arial, Helvetica, sans-serif" font-weight="bold" font-size="28" fill="#0b1220" text-anchor="middle">Add to Slack</text>
</svg>
"""

(HERE / "og-image.svg").write_text(svg)
print("wrote og-image.svg")
