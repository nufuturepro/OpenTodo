#!/bin/sh
# Merge an upstream OpenTodo release into the fork, or re-base the fork onto it when
# upstream re-created its history (FORKING.md → Syncing an upstream release).
# Usage: scripts/sync-upstream.sh v1.6.0 [fork-side-branch]
set -eu
tag=${1:?usage: sync-upstream.sh <upstream-tag, e.g. v1.6.0> [fork-side-branch]}
base_override=${2:-}

# The paths the fork's own work lives in. A re-base replays only these; generated and
# translated files are handled by hand afterwards (FORKING.md → the cutover section).
# Deliberately unquoted where used: the paths are split into arguments.
fork_paths='web/src/app web/src/lib web/src/index.css web/src/globals.d.ts web/vite.config.ts internal/sync/ops.go'

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
here=$(git branch --show-current)

if git merge-base --is-ancestor "$tag" HEAD; then
  echo "$tag is already in $here; nothing to merge"
  exit 0
fi

if [ -n "$(git merge-base HEAD "$tag" 2>/dev/null || true)" ]; then
  if git rev-parse --verify --quiet "refs/tags/upstream-$tag" >/dev/null; then
    echo "note: upstream-$tag already exists; skipping the tag step"
  else
    git tag "upstream-$tag" "$tag"
    echo "tagged upstream-$tag at $tag"
  fi

  git checkout main
  if ! git merge --no-ff "$tag" -m "Merge upstream $tag"; then
    echo ""
    echo "Merge conflicts. Resolve them (take upstream's side where this fork's change"
    echo "no longer applies; re-apply ours on top where it still matters), then:"
    echo "  1. make check   (CI on ubuntu is authoritative on Windows)"
    echo "  2. git push origin main"
    exit 1
  fi

  cat <<EOF

Merged $tag into main.

Next:
  1. Note the sync in CHANGELOG.md and run: make check
  2. Tag a fork release:
       git tag opentodo-$version-1 && git push fork main --tags
EOF
  exit 0
fi

# No common ancestor: upstream re-created its repository at this tag. Merge is
# impossible, so replay the fork's delta from the upstream tag its previous release
# branch was built on onto the new tag.
fork_head=$(git rev-parse HEAD)
base=${base_override:-$here}
if [ -z "$base" ]; then
  base=$(git for-each-ref --sort=-committerdate --count=1 --format='%(refname:short)' 'refs/heads/release-*')
fi
if [ -z "$base" ]; then
  echo "error: no branch to take the fork's delta from; pass the fork side as the second argument" >&2
  exit 1
fi
if ! base_tag=$(git describe --tags --abbrev=0 "$base"); then
  echo "error: $base has no upstream tag to measure the fork's delta from" >&2
  exit 1
fi

echo "upstream re-created its history at $tag (no common ancestor with HEAD)"
echo "re-basing $base onto $tag: delta $base_tag..$base over the fork's paths"

patch=$(git rev-parse --git-dir)/sync-$tag.patch
git diff "$base" "$fork_head" -- $fork_paths > "$patch"
git checkout -B "release-$version" "$tag"
if ! git apply -3 --index "$patch"; then
  echo ""
  echo "The re-apply stopped with conflicts (patch kept at $patch). Resolve them, then:"
  echo "  1. Locales: keep upstream's catalogs and append the fork's new strings by hand."
  echo "  2. UPDATE_OP_SCHEMA=1 go test ./internal/sync -run TestGeneratedOpSchema"
  echo "  3. make check"
  echo "  4. git branch -f main release-$version && git checkout main"
  echo "     git push --force-with-lease fork main release-$version"
  exit 1
fi

if git rev-parse --verify --quiet "refs/tags/upstream-$tag" >/dev/null; then
  echo "note: upstream-$tag already exists; skipping the tag step"
else
  git tag "upstream-$tag" "$tag"
fi

cat <<EOF

Re-based the fork from $base onto $tag on branch release-$version (patch: $patch).

Next:
  1. Locales: keep upstream's catalogs and append the fork's new strings by hand.
  2. UPDATE_OP_SCHEMA=1 go test ./internal/sync -run TestGeneratedOpSchema
  3. make check   (CI on ubuntu is authoritative on Windows)
  4. Adopt the new line when it is green:
       git branch -f main release-$version && git checkout main
       git push --force-with-lease fork main release-$version
EOF
