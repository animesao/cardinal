#!/usr/bin/env python3
"""Check relative links in repository Markdown files.

Usage:
    python3 scripts/check-docs-links.py [repo-root]

Scans Markdown files visible to git (tracked plus untracked, excluding ignored
and vendored files), skips fenced code blocks, and verifies that every relative
link target exists on disk. External URLs, mail links, and pure anchors are
ignored. This is a file-existence check only; it does not validate heading
anchors.
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote

LINK_RE = re.compile(r"\[[^\]]*\]\(([^)\s]+)(?:\s+[\"'][^\"']*[\"'])?\)")
SKIP_PREFIXES = ("http://", "https://", "mailto:", "tel:", "data:", "//")
FENCE_RE = re.compile(r"^\s*(```|~~~)")


def repo_root(argv: list[str]) -> Path:
    if len(argv) > 1:
        return Path(argv[1]).resolve()
    try:
        out = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            check=True,
            capture_output=True,
            text=True,
        )
        return Path(out.stdout.strip()).resolve()
    except (OSError, subprocess.CalledProcessError):
        return Path(__file__).resolve().parent.parent


def markdown_files(root: Path) -> list[Path]:
    try:
        out = subprocess.run(
            [
                "git",
                "-C",
                str(root),
                "ls-files",
                "--cached",
                "--others",
                "--exclude-standard",
                "*.md",
            ],
            check=True,
            capture_output=True,
            text=True,
        )
        return [
            root / line
            for line in dict.fromkeys(out.stdout.splitlines())
            if line and "vendor" not in Path(line).parts
        ]
    except (OSError, subprocess.CalledProcessError):
        return sorted(
            path
            for path in root.rglob("*.md")
            if ".git" not in path.parts and "vendor" not in path.parts
        )


def strip_anchor(target: str) -> str:
    return target.split("#", 1)[0].split("?", 1)[0]


def links_in(path: Path) -> list[tuple[int, str]]:
    links: list[tuple[int, str]] = []
    in_fence = False
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        for match in LINK_RE.finditer(line):
            links.append((number, match.group(1)))
    return links


def resolve(root: Path, source: Path, target: str) -> Path | None:
    if target.startswith(SKIP_PREFIXES) or target.startswith("#"):
        return None
    if target.startswith("<") and target.endswith(">"):
        target = target[1:-1]
    target = strip_anchor(target)
    if not target:
        return None
    target = unquote(target)
    if target.startswith("/"):
        return root / target.lstrip("/")
    return (source.parent / target).resolve()


def main(argv: list[str]) -> int:
    root = repo_root(argv)
    missing: list[str] = []
    checked = 0
    for source in markdown_files(root):
        if not source.is_file():
            continue
        for number, target in links_in(source):
            resolved = resolve(root, source, target)
            if resolved is None:
                continue
            checked += 1
            if not resolved.exists():
                try:
                    shown = source.relative_to(root)
                except ValueError:
                    shown = source
                missing.append(f"{shown}:{number}: {target}")

    if missing:
        print("missing relative link targets:", file=sys.stderr)
        for item in missing:
            print(f"  {item}", file=sys.stderr)
        print(f"checked {checked} local links, {len(missing)} missing", file=sys.stderr)
        return 1

    print(f"checked {checked} local links, all targets resolve")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
