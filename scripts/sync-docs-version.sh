#!/bin/sh
# Synchronize project documentation from the root VERSION file.
# Historical changelog entries remain manual; only marked/generated current blocks change.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION=$(tr -d '[:space:]' < "$ROOT/VERSION")

case "$VERSION" in
  ''|*[!0-9.\-A-Za-z]*)
    echo "Invalid VERSION value: $VERSION" >&2
    exit 1
    ;;
esac

FILES=$(mktemp)
trap 'rm -f "$FILES"' 0 1 2 3 15
find "$ROOT" -type f -name '*.md' -not -path "$ROOT/.git/*" -print > "$FILES"

sync_version_marker() {
  path=$1
  start_count=$(grep -cF '<!-- dck-version:start -->' "$path" || true)
  end_count=$(grep -cF '<!-- dck-version:end -->' "$path" || true)
  if [ "$start_count" -gt 0 ] || [ "$end_count" -gt 0 ]; then
    if [ "$start_count" -ne 1 ] || [ "$end_count" -ne 1 ]; then
      echo "Malformed version markers in ${path#"$ROOT/"}" >&2
      return 1
    fi
  fi

  tmp=$(mktemp "$path.tmp.XXXXXX")
  if [ "$start_count" -eq 0 ]; then
    {
      printf '%s\n' '<!-- dck-version:start -->'
      printf '**Documentation version:** `%s`\n' "$VERSION"
      printf '**Project release:** `v%s`\n' "$VERSION"
      printf '%s\n\n' '<!-- dck-version:end -->'
      cat "$path"
    } > "$tmp"
  else
    awk -v version="$VERSION" '
      /<!-- dck-version:start -->/ {
        print
        print "**Documentation version:** `" version "`"
        print "**Project release:** `v" version "`"
        inside=1
        next
      }
      /<!-- dck-version:end -->/ {
        print
        inside=0
        next
      }
      !inside { print }
    ' "$path" > "$tmp"
  fi

  if ! cmp -s "$tmp" "$path"; then mv "$tmp" "$path"; else rm -f "$tmp"; fi
}

sync_changelog_current_release() {
  path=$1
  start_count=$(grep -cF '<!-- dck-current-release:start -->' "$path" || true)
  end_count=$(grep -cF '<!-- dck-current-release:end -->' "$path" || true)
  if [ "$start_count" -gt 0 ] || [ "$end_count" -gt 0 ]; then
    if [ "$start_count" -ne 1 ] || [ "$end_count" -ne 1 ]; then
      echo "Malformed current-release markers in ${path#"$ROOT/"}" >&2
      return 1
    fi
  fi

  tmp=$(mktemp "$path.tmp.XXXXXX")
  if [ "$start_count" -eq 0 ]; then
    if ! grep -qF '# Changelog' "$path"; then
      echo "Missing changelog heading in ${path#"$ROOT/"}" >&2
      rm -f "$tmp"
      return 1
    fi
    awk -v version="$VERSION" '
      {
        print
        if ($0 == "# Changelog") {
          print ""
          print "<!-- dck-current-release:start -->"
          print "> Current release: **v" version "**. Detailed release notes below are maintained manually."
          print "<!-- dck-current-release:end -->"
        }
      }
    ' "$path" > "$tmp"
  else
    awk -v version="$VERSION" '
      /<!-- dck-current-release:start -->/ {
        print
        print "> Current release: **v" version "**. Detailed release notes below are maintained manually."
        inside=1
        next
      }
      /<!-- dck-current-release:end -->/ {
        print
        inside=0
        next
      }
      !inside { print }
    ' "$path" > "$tmp"
  fi

  if ! cmp -s "$tmp" "$path"; then mv "$tmp" "$path"; else rm -f "$tmp"; fi
}

sync_readme_release_block() {
  path=$1
  start_count=$(grep -cF '<!-- dck-release:start -->' "$path" || true)
  end_count=$(grep -cF '<!-- dck-release:end -->' "$path" || true)
  if [ "$start_count" -gt 0 ] || [ "$end_count" -gt 0 ]; then
    if [ "$start_count" -ne 1 ] || [ "$end_count" -ne 1 ]; then
      echo "Malformed release markers in ${path#"$ROOT/"}" >&2
      return 1
    fi
  fi

  tmp=$(mktemp "$path.tmp.XXXXXX")
  if [ "$start_count" -eq 0 ]; then
    if ! grep -qF '## Changelog' "$path"; then
      echo "Missing README changelog heading in ${path#"$ROOT/"}" >&2
      rm -f "$tmp"
      return 1
    fi
    awk -v version="$VERSION" '
      {
        print
        if ($0 == "## Changelog") {
          print ""
          print "<!-- dck-release:start -->"
          print "**v" version "** — Documentation, installation, AppImage, update, and release automation are synchronized from the root `VERSION` file."
          print "<!-- dck-release:end -->"
        }
      }
    ' "$path" > "$tmp"
  else
    awk -v version="$VERSION" '
      /<!-- dck-release:start -->/ {
        print
        print "**v" version "** — Documentation, installation, AppImage, update, and release automation are synchronized from the root `VERSION` file."
        inside=1
        next
      }
      /<!-- dck-release:end -->/ {
        print
        inside=0
        next
      }
      !inside { print }
    ' "$path" > "$tmp"
  fi

  if ! cmp -s "$tmp" "$path"; then mv "$tmp" "$path"; else rm -f "$tmp"; fi
}

check_version_marker() {
  path=$1
  start_count=$(grep -cF '<!-- dck-version:start -->' "$path" || true)
  end_count=$(grep -cF '<!-- dck-version:end -->' "$path" || true)
  if [ "$start_count" -ne 1 ] || [ "$end_count" -ne 1 ]; then
    echo "Malformed or missing version markers in ${path#"$ROOT/"}" >&2
    return 1
  fi
  block=$(sed -n '/<!-- dck-version:start -->/,/<!-- dck-version:end -->/p' "$path")
  expected="**Documentation version:** \`$VERSION\`"
  expected_release="**Project release:** \`v$VERSION\`"
  actual=$(printf '%s\n' "$block" | sed -n '2p')
  actual_release=$(printf '%s\n' "$block" | sed -n '3p')
  if [ "$actual" != "$expected" ] || [ "$actual_release" != "$expected_release" ]; then
    echo "Stale documentation version in ${path#"$ROOT/"}" >&2
    return 1
  fi
}

check_changelog_current_release() {
  path=$1
  start_count=$(grep -cF '<!-- dck-current-release:start -->' "$path" || true)
  end_count=$(grep -cF '<!-- dck-current-release:end -->' "$path" || true)
  if [ "$start_count" -ne 1 ] || [ "$end_count" -ne 1 ]; then
    echo "Malformed or missing current-release markers in ${path#"$ROOT/"}" >&2
    return 1
  fi
  if ! sed -n '/<!-- dck-current-release:start -->/,/<!-- dck-current-release:end -->/p' "$path" | grep -q "v$VERSION"; then
    echo "Stale current changelog release in ${path#"$ROOT/"}" >&2
    return 1
  fi
}

check_readme_release_block() {
  path=$1
  start_count=$(grep -cF '<!-- dck-release:start -->' "$path" || true)
  end_count=$(grep -cF '<!-- dck-release:end -->' "$path" || true)
  if [ "$start_count" -ne 1 ] || [ "$end_count" -ne 1 ]; then
    echo "Malformed or missing release markers in ${path#"$ROOT/"}" >&2
    return 1
  fi
  if ! sed -n '/<!-- dck-release:start -->/,/<!-- dck-release:end -->/p' "$path" | grep -q "v$VERSION"; then
    echo "Stale current release in ${path#"$ROOT/"}" >&2
    return 1
  fi
}

mode=update
if [ "${1:-}" = "--check" ]; then
  mode=check
elif [ "${1:-}" != "" ]; then
  echo "Usage: $0 [--check]" >&2
  exit 2
fi

status=0
while IFS= read -r path; do
  if [ "$mode" = update ]; then
    sync_version_marker "$path" || status=1
  else
    check_version_marker "$path" || status=1
  fi
done < "$FILES"

if [ "$mode" = update ]; then
  sync_readme_release_block "$ROOT/README.md" || status=1
  sync_changelog_current_release "$ROOT/CHANGELOG.md" || status=1
else
  check_readme_release_block "$ROOT/README.md" || status=1
  check_changelog_current_release "$ROOT/CHANGELOG.md" || status=1
fi

[ "$status" -eq 0 ] || exit "$status"
if [ "$mode" = update ]; then
  echo "Documentation version synchronized to $VERSION"
else
  echo "Documentation version markers are synchronized to $VERSION"
fi
