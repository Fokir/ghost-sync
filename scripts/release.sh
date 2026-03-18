#!/usr/bin/env bash
set -euo pipefail

# release.sh — Determine next semver tag from commit history and push it.
#
# Usage:
#   ./scripts/release.sh           # auto-detect bump type from commits
#   ./scripts/release.sh patch     # force patch bump
#   ./scripts/release.sh minor     # force minor bump
#   ./scripts/release.sh major     # force major bump
#   DRY_RUN=1 ./scripts/release.sh # show what would happen

DRY_RUN="${DRY_RUN:-0}"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

die() { echo "ERROR: $*" >&2; exit 1; }

# Get the latest semver tag (vX.Y.Z). Returns "v0.0.0" if none exist.
latest_tag() {
  git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1
}

# Parse semver components from a vX.Y.Z string.
parse_semver() {
  local tag="${1#v}"
  IFS='.' read -r MAJOR MINOR PATCH <<< "$tag"
}

# Determine bump type from conventional commit messages since the last tag.
# Rules:
#   - Any commit starting with "feat" or containing "BREAKING CHANGE" → at least minor
#   - "BREAKING CHANGE" or "!" after type → major
#   - Everything else (fix, chore, ci, docs, test, refactor) → patch
detect_bump() {
  local since="$1"
  local range
  if [ "$since" = "v0.0.0" ]; then
    range="HEAD"
  else
    range="${since}..HEAD"
  fi

  local bump="patch"
  while IFS= read -r msg; do
    # Major: BREAKING CHANGE anywhere or type! (e.g., "feat!:")
    if echo "$msg" | grep -qiE '(BREAKING CHANGE|^[a-z]+!\s*:)'; then
      echo "major"
      return
    fi
    # Minor: feat commits
    if echo "$msg" | grep -qiE '^feat(\(|:|\!)'; then
      bump="minor"
    fi
  done < <(git log "$range" --pretty=format:"%s" 2>/dev/null)

  echo "$bump"
}

# Compute next version given current version and bump type.
next_version() {
  local current="$1" bump="$2"
  parse_semver "$current"

  case "$bump" in
    major) echo "v$((MAJOR + 1)).0.0" ;;
    minor) echo "v${MAJOR}.$((MINOR + 1)).0" ;;
    patch) echo "v${MAJOR}.${MINOR}.$((PATCH + 1))" ;;
    *)     die "unknown bump type: $bump" ;;
  esac
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

# Ensure we are in a git repo with a clean working tree.
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a git repository"

if [ -n "$(git status --porcelain)" ]; then
  die "working tree is dirty — commit or stash changes first"
fi

# Ensure we are on master/main.
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$BRANCH" != "master" && "$BRANCH" != "main" ]]; then
  die "releases must be created from master or main (currently on '$BRANCH')"
fi

# Ensure local is up to date with remote.
git fetch --tags origin 2>/dev/null || true

CURRENT="$(latest_tag)"
[ -z "$CURRENT" ] && CURRENT="v0.0.0"

# Determine bump type: from argument or auto-detect.
if [ $# -ge 1 ]; then
  BUMP="$1"
  case "$BUMP" in
    major|minor|patch) ;;
    *) die "invalid bump type '$BUMP' — use major, minor, or patch" ;;
  esac
else
  BUMP="$(detect_bump "$CURRENT")"
fi

NEXT="$(next_version "$CURRENT" "$BUMP")"

echo ""
echo "  Current version : $CURRENT"
echo "  Bump type       : $BUMP"
echo "  Next version    : $NEXT"
echo ""

# Show commits that will be in this release.
if [ "$CURRENT" = "v0.0.0" ]; then
  echo "  Commits (all):"
  git log --oneline --no-decorate | sed 's/^/    /'
else
  echo "  Commits since $CURRENT:"
  git log "${CURRENT}..HEAD" --oneline --no-decorate | sed 's/^/    /'
fi
echo ""

if [ "$DRY_RUN" = "1" ]; then
  echo "  [DRY RUN] Would create and push tag $NEXT"
  exit 0
fi

# Confirm.
read -r -p "  Create and push tag $NEXT? [y/N] " confirm
case "$confirm" in
  [yY]|[yY][eE][sS]) ;;
  *) echo "  Aborted."; exit 0 ;;
esac

# Create annotated tag and push.
git tag -a "$NEXT" -m "Release $NEXT"
git push origin "$NEXT"

echo ""
echo "  Tag $NEXT pushed. GitHub Actions will build and publish the release."
echo "  Watch: gh run list --repo $(git remote get-url origin | sed 's/.*github.com[:/]//' | sed 's/\.git$//')"
echo ""
