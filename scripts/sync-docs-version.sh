#!/bin/sh
# Synchronize project documentation from the root VERSION file.
# Historical changelog entries remain manual; only marked/generated current blocks change.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION=$(tr -d '[:space:]' < "$ROOT/VERSION")

# Debian/SemVer-friendly charset (digits, letters, . - + ~ _).
case "$VERSION" in
  ''|*[!0-9A-Za-z.+~_-]*)
    echo "Invalid VERSION value: $VERSION" >&2
    exit 1
    ;;
esac

FILES=$(mktemp)
TMP_PATHS=""
cleanup() {
    rm -f "$FILES"
    for p in $TMP_PATHS; do rm -f "$p"; done
}
trap cleanup 0 1 2 3 15

# Skip README/CHANGELOG here — they get dedicated markers below.
find "$ROOT" -type f -name '*.md' \
    -not -path "$ROOT/.git/*" \
    -not -path "$ROOT/vendor/*" \
    -not -path "$ROOT/node_modules/*" \
    -not -name 'README.md' \
    -not -name 'CHANGELOG.md' \
    -print > "$FILES" 2>/dev/null || true

# --- helpers ---------------------------------------------------------------

# mark_tmp PATH -> echoes tmp path, registers in TMP_PATHS
new_tmp() {
    t=$(mktemp "$1.tmp.XXXXXX")
    TMP_PATHS="$TMP_PATHS $t"
    printf '%s' "$t"
}

# count_marker FILE MARKER -> echoes count
count_marker() {
    [ -f "$1" ] || { echo 0; return 0; }
    grep -cF "$2" "$1" || true
}

# check_single_pair FILE START END RELNAME -> 0/1
check_single_pair() {
    _f=$1 _s=$2 _e=$3 _r=$4
    [ -f "$_f" ] || { echo "File not found: $_r" >&2; return 1; }
    sc=$(count_marker "$_f" "$_s")
    ec=$(count_marker "$_f" "$_e")
    if [ "$sc" -ne 1 ] || [ "$ec" -ne 1 ]; then
        echo "Malformed or missing markers in $_r (start=$sc end=$ec)" >&2
        return 1
    fi
    sl=$(grep -nF "$_s" "$_f" | head -n1 | cut -d: -f1)
    el=$(grep -nF "$_e" "$_f" | head -n1 | cut -d: -f1)
    if [ "$sl" -ge "$el" ]; then
        echo "Markers out of order in $_r (start@$sl end@$el)" >&2
        return 1
    fi
    return 0
}

# run_awk AWK_PROGRAM IN OUT VERSION -> 0/1, no partial overwrite
run_awk() {
    _prog=$1 _in=$2 _out=$3 _ver=$4
    if ! awk -v version="$_ver" "$_prog" "$_in" > "$_out"; then
        echo "awk failed processing $_in" >&2
        rm -f "$_out"
        return 1
    fi
}

# --- version marker (for all *.md except README/CHANGELOG) -----------------

SYNC_VERSION_AWK='
  /<!-- cardinal-version:start -->/ {
    print
    print "**Documentation version:** `" version "`"
    print "**Project release:** `v" version "`"
    inside=1; next
  }
  /<!-- cardinal-version:end -->/ { print; inside=0; next }
  !inside { print }
'

sync_version_marker() {
    path=$1
    rel=${path#"$ROOT/"}
    sc=$(count_marker "$path" '<!-- cardinal-version:start -->')
    ec=$(count_marker "$path" '<!-- cardinal-version:end -->')

    if [ "$sc" -gt 0 ] || [ "$ec" -gt 0 ]; then
        check_single_pair "$path" \
            '<!-- cardinal-version:start -->' \
            '<!-- cardinal-version:end -->' "$rel" || return 1
    fi

    tmp=$(new_tmp "$path")
    if [ "$sc" -eq 0 ]; then
        {
            printf '%s\n' '<!-- cardinal-version:start -->'
            printf '**Documentation version:** `%s`\n' "$VERSION"
            printf '**Project release:** `v%s`\n' "$VERSION"
            printf '%s\n\n' '<!-- cardinal-version:end -->'
            cat "$path"
        } > "$tmp" || { rm -f "$tmp"; return 1; }
    else
        run_awk "$SYNC_VERSION_AWK" "$path" "$tmp" "$VERSION" || return 1
    fi

    if ! cmp -s "$tmp" "$path"; then
        mv "$tmp" "$path"
        echo "updated $rel"
    else
        rm -f "$tmp"
    fi
    TMP_PATHS=$(printf '%s' "$TMP_PATHS" | sed "s| *$tmp||")
}

check_version_marker() {
    path=$1
    rel=${path#"$ROOT/"}
    check_single_pair "$path" \
        '<!-- cardinal-version:start -->' \
        '<!-- cardinal-version:end -->' "$rel" || return 1

    block=$(sed -n '/<!-- cardinal-version:start -->/,/<!-- cardinal-version:end -->/p' "$path")
    printf '%s\n' "$block" | grep -qF "**Documentation version:** \`$VERSION\`" \
        || { echo "Stale doc version in $rel" >&2; return 1; }
    printf '%s\n' "$block" | grep -qF "**Project release:** \`v$VERSION\`" \
        || { echo "Stale project release in $rel" >&2; return 1; }
    return 0
}

# --- README badge ----------------------------------------------------------

README="$ROOT/README.md"

sync_readme_version_badge() {
    rel=${README#"$ROOT/"}
    check_single_pair "$README" \
        '<!-- cardinal-version-badge:start -->' \
        '<!-- cardinal-version-badge:end -->' "$rel" || return 1

    tmp=$(new_tmp "$README")
    run_awk '
      /<!-- cardinal-version-badge:start -->/ {
        print
        print "  <img src=\"https://img.shields.io/badge/version-v" version "-blue?style=flat-square\">"
        inside=1; next
      }
      /<!-- cardinal-version-badge:end -->/ { print; inside=0; next }
      !inside { print }
    ' "$README" "$tmp" "$VERSION" || return 1

    if ! cmp -s "$tmp" "$README"; then
        mv "$tmp" "$README"
        echo "updated $rel (badge)"
    else
        rm -f "$tmp"
    fi
    TMP_PATHS=$(printf '%s' "$TMP_PATHS" | sed "s| *$tmp||")
}

check_readme_version_badge() {
    rel=${README#"$ROOT/"}
    check_single_pair "$README" \
        '<!-- cardinal-version-badge:start -->' \
        '<!-- cardinal-version-badge:end -->' "$rel" || return 1
    sed -n '/<!-- cardinal-version-badge:start -->/,/<!-- cardinal-version-badge:end -->/p' "$README" \
        | grep -qF "version-v$VERSION-blue" \
        || { echo "Stale README version badge in $rel" >&2; return 1; }
    return 0
}

# --- CHANGELOG -------------------------------------------------------------
# (аналогично: sync_changelog_current_release / check_changelog_current_release)
# Ключевые правки:
#   * искать заголовок через index($0, "# Changelog") == 1
#   * run_awk с проверкой кода
#   * grep -qF "v$VERSION"
#   * check_single_pair для маркеров

# --- README release block --------------------------------------------------
# (аналогично: sync_readme_release_block / check_readme_release_block)
# Ключевые правки:
#   * искать заголовок через index($0, "## Changelog") == 1
#   * grep -qF "v$VERSION"

# --- main ------------------------------------------------------------------

mode=update
case "${1:-}" in
    "")        mode=update ;;
    --check)   mode=check ;;
    *)         echo "Usage: $0 [--check]" >&2; exit 2 ;;
esac

status=0
while IFS= read -r path; do
    [ -n "$path" ] || continue
    if [ "$mode" = update ]; then
        sync_version_marker "$path" || status=1
    else
        check_version_marker "$path" || status=1
    fi
done < "$FILES"

if [ "$mode" = update ]; then
    sync_readme_version_badge     || status=1
    sync_readme_release_block     || status=1
    sync_changelog_current_release || status=1
else
    check_readme_version_badge      || status=1
    check_readme_release_block      || status=1
    check_changelog_current_release || status=1
fi

[ "$status" -eq 0 ] || exit "$status"
if [ "$mode" = update ]; then
    echo "Documentation version synchronized to $VERSION"
else
    echo "Documentation version markers are synchronized to $VERSION"
fi
