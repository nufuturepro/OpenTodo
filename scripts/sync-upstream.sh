#!/bin/sh
# Merge an upstream OpenTodo release into main (FORKING.md → Syncing an upstream release).
# Usage: scripts/sync-upstream.sh v0.8.1
set -eu
tag=${1:?usage: sync-upstream.sh <upstream-tag, e.g. v0.8.1>}

if [ -n "$(git status --porcelain)" ]; then
  echo "error: working tree is dirty; commit or stash first" >&2
  exit 1
fi

git fetch upstream --tags
if ! git rev-parse --verify --quiet "refs/tags/$tag" >/dev/null; then
  echo "error: no such upstream tag: $tag (fetch brought: $(git tag -l 'v*' | tail -3 | tr '\n' ' '))" >&2
  exit 1
fi

version=${tag#v}
if git rev-parse --verify --quiet "refs/tags/upstream-$tag" >/dev/null; then
  echo "note: upstream-$tag already exists; skipping the tag step"
else
  git tag "upstream-$tag" "$tag"
  echo "tagged upstream-$tag at $tag"
fi

git checkout main
git merge --no-ff "$tag" -m "Merge upstream $tag" || {
  echo ""
  echo "Merge conflicts. Resolve them (take upstream's side where this fork's change"
  echo "no longer applies; re-apply ours on top where it still matters), then:"
  echo "  1. make check   (CI on ubuntu is authoritative on Windows)"
  echo "  2. git push origin main"
  exit 1
}

cat <<EOF

Merged $tag into main.

Next:
  1. Note the sync in CHANGELOG.md and run: make check
  2. Tag a fork release:
       git tag opentodo-$version-1 && git push origin main --tags
EOF
