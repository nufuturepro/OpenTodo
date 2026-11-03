#!/bin/sh
# Propose commits upstream: branch off upstream/main, cherry-pick, reset the fork's
# changelog entries, push to the thin fork and open the PR against pixari/OpenTodo
# (FORKING.md → Sending a bug fix upstream).
# Usage: scripts/upstream-pr.sh <branch-name> <commit>...
#        STRIP_ONLY=1 scripts/upstream-pr.sh   # the changelog pass alone, after a
#                                               # conflict was resolved by hand
set -eu
if [ -n "$(git status --porcelain)" ]; then
  echo "error: the tree is dirty; commit or stash first (the pass rewrites files in place)" >&2
  exit 1
fi

# Present the proposed change as upstream would: the changelog goes back to
# upstream's (its entries and release notes are written there), and the branch must
# end with no OpenTaskKit string anywhere in its diff — the fork speaks upstream's
# name, so a surviving brand string means a stale file that would need reformatting.
prepare_for_upstream() {
  base=$(git merge-base HEAD upstream/main)
  changed=$(git diff --name-only "$base"..HEAD)
  if printf '%s\n' "$changed" | grep -qx 'CHANGELOG.md'; then
    git checkout upstream/main -- CHANGELOG.md
    echo "  CHANGELOG.md reset to upstream's (its release notes are upstream's)"
  fi
  leftover=$(printf '%s\n' "$changed" | while IFS= read -r f; do
    [ -f "$f" ] && grep -l 'OpenTaskKit' "$f" 2>/dev/null || true
  done)
  if [ -n "$leftover" ]; then
    echo "error: OpenTaskKit still present in the branch:" >&2
    printf '%s\n' "$leftover" >&2
    exit 1
  fi
  if ! git diff --quiet || ! git diff --cached --quiet; then
    git add -A
    if [ "$(git rev-list --count upstream/main..HEAD)" -eq 1 ]; then
      git commit --amend --no-edit --quiet
    else
      git commit -m "chore: reset the changelog for upstream" --quiet
    fi
    echo "  changelog reset folded into the branch"
  fi
}

if [ "${STRIP_ONLY:-}" = "1" ]; then
  prepare_for_upstream
  exit 0
fi

name=${1:?usage: upstream-pr.sh <branch-name> <commit>...}
shift
if [ $# -eq 0 ]; then
  echo "error: give the commit(s) to send upstream" >&2
  exit 1
fi

git fetch upstream
here=$(git branch --show-current)
branch="upstream-pr/$name"
git checkout -B "$branch" upstream/main
if ! git cherry-pick "$@"; then
  echo ""
  echo "Cherry-pick conflict. Resolve it, then:"
  echo "  git cherry-pick --continue"
  echo "  STRIP_ONLY=1 scripts/upstream-pr.sh   # the changelog pass"
  echo "  git push fork $branch"
  echo "  gh pr create -R pixari/OpenTodo --base main --head nufuturepro:$branch --title '$name'"
  exit 1
fi
prepare_for_upstream

git push -u fork "$branch"

summary=$(git log --format='- %s' "upstream/main..HEAD" | sed 's/^/  /')
gh pr create -R pixari/OpenTodo --base main --head "nufuturepro:$branch" \
  --title "$name" \
  --body "Proposed from the [nufuturepro/OpenTaskKit fork](https://github.com/nufuturepro/OpenTaskKit); it lands there with the next upstream sync.

Commits:

$summary
"
echo ""
echo "PR opened for $branch. Return to $here with: git checkout $here"
