#!/usr/bin/env python3
"""Convert CHANGELOG.md to site/changelog.html matching the muxwarp site theme."""

import re
import sys
from pathlib import Path


def parse_changelog(md: str) -> str:
    """Parse CHANGELOG.md into HTML body content."""
    lines = md.strip().split("\n")
    html_parts: list[str] = []
    in_list = False
    in_version = False

    for line in lines:
        stripped = line.strip()

        # Skip the title and format/versioning preamble
        if stripped.startswith("# Changelog"):
            continue
        if stripped.startswith("Format:") or stripped.startswith("Versioning:"):
            continue
        # Skip link references at bottom
        if re.match(r"^\[.+?\]:", stripped):
            continue
        if not stripped:
            if in_list:
                html_parts.append("</ul>")
                in_list = False
            continue

        # Version headers: ## [0.2.0] - 2026-03-27
        m = re.match(r"^## \[(.+?)\] - (\d{4}-\d{2}-\d{2})$", stripped)
        if m:
            if in_list:
                html_parts.append("</ul>")
                in_list = False
            if in_version:
                html_parts.append("</div>")
            in_version = True
            ver, date = m.group(1), m.group(2)
            html_parts.append(
                f'<div class="version-block">'
                f'<div class="version-header">'
                f'<h2>v{ver}</h2>'
                f'<span class="version-date">{date}</span>'
                f"</div>"
            )
            continue

        # Section headers: ### Added, ### Changed, etc.
        m = re.match(r"^### (.+)$", stripped)
        if m:
            if in_list:
                html_parts.append("</ul>")
                in_list = False
            label = m.group(1)
            css_class = label.lower()
            html_parts.append(
                f'<span class="change-type change-{css_class}">{label}</span>'
            )
            continue

        # List items
        if stripped.startswith("- "):
            if not in_list:
                html_parts.append("<ul>")
                in_list = True
            content = inline_markup(stripped[2:])
            html_parts.append(f"<li>{content}</li>")
            continue

        # Continuation lines (indented list items)
        if stripped and in_list:
            content = inline_markup(stripped)
            if html_parts and html_parts[-1].startswith("<li>"):
                html_parts[-1] = html_parts[-1][:-5] + " " + content + "</li>"
            continue

    if in_list:
        html_parts.append("</ul>")
    if in_version:
        html_parts.append("</div>")

    return "\n".join(html_parts)


def inline_markup(text: str) -> str:
    """Convert **bold** and `code` to HTML."""
    text = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", text)
    text = re.sub(r"`(.+?)`", r"<code>\1</code>", text)
    return text


def build_page(body: str) -> str:
    """Wrap changelog body in the full HTML page."""
    return f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Changelog - muxwarp</title>
  <meta name="description" content="muxwarp changelog — all notable changes by version.">
  <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>&#9650;</text></svg>">
  <style>
    @import url('https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;600;700&family=JetBrains+Mono:wght@400;600&display=swap');

    :root {{
      --green-50: #ECFDF5;
      --green-100: #D1FAE5;
      --green-200: #A7F3D0;
      --green-300: #6EE7B7;
      --green-400: #34D399;
      --green-500: #10B981;
      --green-600: #059669;
      --green-700: #047857;
      --green-800: #065F46;
      --green-900: #064E3B;
      --green-950: #022C22;
      --white: #FFFFFF;
      --text: #064E3B;
      --text-muted: #6B8F80;
      --mono: 'JetBrains Mono', 'SF Mono', 'Fira Code', monospace;
    }}

    * {{ margin: 0; padding: 0; box-sizing: border-box; }}

    body {{
      font-family: 'Space Grotesk', -apple-system, system-ui, sans-serif;
      background: var(--white);
      color: var(--text);
      line-height: 1.6;
    }}

    nav {{
      position: fixed;
      top: 0; left: 0; right: 0;
      z-index: 100;
      padding: 16px 32px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      background: rgba(255,255,255,0.85);
      backdrop-filter: blur(12px);
      -webkit-backdrop-filter: blur(12px);
      border-bottom: 1px solid var(--green-200);
    }}

    .nav-logo {{
      display: flex;
      align-items: center;
      gap: 10px;
      text-decoration: none;
      font-weight: 700;
      font-size: 1.1rem;
      color: var(--green-800);
    }}

    .nav-logo .tri {{
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      background: var(--green-500);
      color: white;
      border-radius: 6px;
      font-size: 0.9rem;
      font-weight: 700;
    }}

    .nav-links {{
      display: flex;
      gap: 28px;
      list-style: none;
    }}

    .nav-links a {{
      text-decoration: none;
      color: var(--text-muted);
      font-size: 0.9rem;
      font-weight: 500;
      transition: color 0.2s;
    }}

    .nav-links a:hover {{ color: var(--green-600); }}

    main {{
      max-width: 720px;
      margin: 0 auto;
      padding: 120px 24px 80px;
    }}

    .page-header {{
      margin-bottom: 48px;
    }}

    .page-header .label {{
      display: inline-block;
      font-size: 0.75rem;
      font-weight: 700;
      letter-spacing: 2px;
      text-transform: uppercase;
      color: var(--green-500);
      margin-bottom: 8px;
    }}

    .page-header h1 {{
      font-size: 2.4rem;
      font-weight: 700;
      letter-spacing: -1px;
      color: var(--green-950);
    }}

    .page-header p {{
      font-size: 1.05rem;
      color: var(--text-muted);
      margin-top: 8px;
    }}

    .version-block {{
      margin-bottom: 48px;
      padding: 28px;
      background: var(--green-50);
      border: 1px solid var(--green-200);
      border-radius: 16px;
    }}

    .version-header {{
      display: flex;
      align-items: baseline;
      gap: 16px;
      margin-bottom: 20px;
    }}

    .version-header h2 {{
      font-size: 1.5rem;
      font-weight: 700;
      color: var(--green-900);
      font-family: var(--mono);
    }}

    .version-date {{
      font-size: 0.85rem;
      color: var(--text-muted);
      font-family: var(--mono);
    }}

    .change-type {{
      display: inline-block;
      font-size: 0.7rem;
      font-weight: 700;
      letter-spacing: 1.5px;
      text-transform: uppercase;
      padding: 3px 10px;
      border-radius: 6px;
      margin: 16px 0 8px;
    }}

    .change-added {{
      background: var(--green-100);
      color: var(--green-700);
    }}

    .change-changed {{
      background: #FEF3C7;
      color: #92400E;
    }}

    .change-fixed {{
      background: #DBEAFE;
      color: #1E40AF;
    }}

    .change-removed {{
      background: #FEE2E2;
      color: #991B1B;
    }}

    ul {{
      list-style: none;
      margin: 0;
      padding: 0;
    }}

    li {{
      position: relative;
      padding: 6px 0 6px 20px;
      font-size: 0.95rem;
      color: var(--text);
      line-height: 1.55;
    }}

    li::before {{
      content: '\\25B8';
      position: absolute;
      left: 0;
      color: var(--green-400);
      font-size: 0.8rem;
    }}

    li strong {{
      color: var(--green-800);
    }}

    li code {{
      font-family: var(--mono);
      font-size: 0.85em;
      background: var(--green-100);
      padding: 1px 5px;
      border-radius: 4px;
      color: var(--green-700);
    }}

    footer {{
      text-align: center;
      padding: 32px 24px;
      border-top: 1px solid var(--green-200);
      color: var(--text-muted);
      font-size: 0.85rem;
    }}

    footer a {{
      color: var(--green-600);
      text-decoration: none;
    }}

    footer a:hover {{ text-decoration: underline; }}

    @media (max-width: 768px) {{
      nav {{ padding: 12px 16px; }}
      .nav-links {{ gap: 16px; }}
      main {{ padding: 100px 16px 60px; }}
      .page-header h1 {{ font-size: 1.8rem; }}
      .version-block {{ padding: 20px; }}
    }}
  </style>
</head>
<body>
  <nav>
    <a class="nav-logo" href="index.html">
      <span class="tri">&#9650;</span>
      muxwarp
    </a>
    <ul class="nav-links">
      <li><a href="index.html">Home</a></li>
      <li><a href="changelog.html">Changelog</a></li>
      <li><a href="https://github.com/clintecker/muxwarp">GitHub</a></li>
    </ul>
  </nav>

  <main>
    <div class="page-header">
      <span class="label">History</span>
      <h1>Changelog</h1>
      <p>All notable changes to muxwarp, by version.</p>
    </div>

{body}
  </main>

  <footer>
    <p>
      Built by <a href="https://github.com/clintecker">Clint Ecker</a> &middot;
      <a href="https://github.com/clintecker/muxwarp">Source on GitHub</a> &middot;
      MIT License
    </p>
  </footer>
</body>
</html>"""


def main() -> None:
    root = Path(__file__).resolve().parent.parent
    changelog_path = root / "CHANGELOG.md"
    output_path = root / "site" / "changelog.html"

    if not changelog_path.exists():
        print(f"Error: {changelog_path} not found", file=sys.stderr)
        sys.exit(1)

    md = changelog_path.read_text()
    body = parse_changelog(md)
    html = build_page(body)
    output_path.write_text(html)
    print(f"Generated {output_path}")


if __name__ == "__main__":
    main()
