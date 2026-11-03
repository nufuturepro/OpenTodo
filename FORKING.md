# Forking playbook

This repository is a **fork of [OpenTodo](https://github.com/pixari/OpenTodo)**, in the
relationship Debian has with Ubuntu: upstream is where features and general bug fixes are
built; this repository stays close enough to merge upstream releases routinely and carries
this fork's own additions until they are proposed upstream.

## The three repositories

| Repository | Role |
| --- | --- |
| [pixari/OpenTodo](https://github.com/pixari/OpenTodo) | **Upstream.** The product this fork is built from. |
| [nufuturepro/OpenTodo](https://github.com/nufuturepro/OpenTodo) | **Thin fork.** Kept identical to upstream; exists only so bugfix branches can be proposed as PRs to upstream. Never add fork-specific work here. |
| [nufuturepro/OpenTaskKit](https://github.com/nufuturepro/OpenTaskKit) | **This repository.** The fork. Merges upstream releases and publishes `ghcr.io/nufuturepro/opentaskkit` — the image name follows the repository name, which is still the address this fork is published under. |

Local remotes in a clone of this repository:

```
origin   → nufuturepro/OpenTaskKit   (this fork)
upstream → pixari/OpenTodo           (the source of releases)
fork     → nufuturepro/OpenTodo      (the staging area for upstream PRs)
```

## Branches, tags and versions

- `main` is the product. Upstream releases are merged into it with a
  `Merge upstream vX.Y.Z` commit; this fork's changes land on top.
- Every synced upstream release is marked with a local tag **`upstream-vX.Y.Z`**
  (same commit as upstream's `vX.Y.Z`). `git log upstream-vX.Y.Z..HEAD --oneline`
  is this fork's exact delta on top of that release.
- Fork releases are tagged **`opentodo-X.Y.Z-N`** (for example `opentodo-0.19.0-1`)
  and published by CI (container image, binaries, GitHub release). The in-app version
  string comes from `git describe`, so a build from `main` reads like the nearest tag
  plus commits.
- Feature work happens on branches off `main`; merge back with a PR.

## What a sync usually conflicts on

The fork speaks upstream's name everywhere, so there is no brand sweep to re-apply.
What still diverges from upstream and needs a hand during a merge:

| Area | Files |
| --- | --- |
| This fork's features | the grip row menu, the one-time grip hint, the version row and changelog dialog, the animation preference |
| Removed features | the built-in plugins settings section (deleted here; a sync that brings it back is re-removed) |
| Distribution tooling | `FORKING.md`, `scripts/sync-upstream.sh`, `scripts/upstream-pr.sh`, `.github/workflows/sync-upstream.yml` |
| Housekeeping | `CHANGELOG.md` fork entries, `README.md` |

## Syncing an upstream release

Either run the weekly workflow (opens a "Sync upstream vX.Y.Z" PR automatically), or
by hand:

```sh
git fetch upstream --tags
scripts/sync-upstream.sh vX.Y.Z     # tags, merges into main, prints the follow-up steps
```

The script refuses to run with a dirty tree, tags `upstream-vX.Y.Z`, merges with
`--no-ff`, and prints the follow-up steps (`make check`, changelog). Resolve conflicts
by taking upstream's side where this fork's change no longer applies, and re-applying
this fork's change on top where it still matters.

## Sending a bug fix upstream

Generic fixes (a bug anyone can hit, no fork-specific behaviour) go upstream
first; they return with the next sync, conflict-free, because the same change flows
both ways:

```sh
scripts/upstream-pr.sh fix-short-description <commit>...
```

The script branches off `upstream/main`, cherry-picks the commits, resets
`CHANGELOG.md` to upstream's (its release notes are written there), refuses to push if
an `OpenTaskKit` string ever survives in the diff, pushes to the `fork` remote and opens
the PR against `pixari/OpenTodo`. Run it with a clean tree; on a cherry-pick conflict it
stops and prints the manual steps. Follow upstream's CONTRIBUTING: bug fixes need no
OpenSpec change; `make check` must be green there; UI changes need before/after
screenshots from `make screens`.

This fork's own additions stay here until they are proposed upstream.

## Windows development notes

- Set `git config core.autocrlf false` in your clone (this checkout ships LF; several
  golden and docs-parsing tests fail on CRLF).
- Upstream's tests assume POSIX: `os.Rename` over open files, `0600` file modes and
  `flock` semantics. On Windows, `go test ./...` fails in the backup/host/tenant/push/
  settings/store/blobs families and a few web tests (ESLint config resolution, Dexie
  teardown) regardless of any local change — verified on pristine upstream code.
  **GitHub Actions (ubuntu) is the authoritative gate**; always check CI before
  blaming a change.

## Licence and credit

AGPL-3.0, like upstream. Modified distributions must publish their source (this
repository does). Keep the attribution in the README, do not imply upstream
endorsement, and support upstream — it is where the product is made.
