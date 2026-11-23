# Forking playbook

This repository is a **fork of [OpenTodo](https://github.com/OpenTodoApp/OpenTodo)**, in the
relationship Debian has with Ubuntu: upstream is where features and general bug fixes are
built; this repository stays close enough to take upstream releases routinely and carries
this fork's own additions until they are proposed upstream.

## The repositories

| Repository | Role |
| --- | --- |
| [OpenTodoApp/OpenTodo](https://github.com/OpenTodoApp/OpenTodo) | **Upstream.** The product this fork is built from. The old address `pixari/OpenTodo` redirects here. |
| [nufuturepro/OpenTodo](https://github.com/nufuturepro/OpenTodo) | **This fork on GitHub.** `main`, the `release-*` lines and the `upstream-pr/*` branches live here, and releases are pushed here. |
| [nufuturepro/OpenTaskKit](https://github.com/nufuturepro/OpenTaskKit) | The fork's earlier home (`origin` in existing clones), last pushed in October 2026. Kept as a remote for its history. |

Local remotes in a clone of this repository:

```
origin   → nufuturepro/OpenTaskKit   (the fork's earlier home)
fork     → nufuturepro/OpenTodo      (this fork's GitHub home; branches and releases go here)
upstream → pixari/OpenTodo           (upstream; redirects to OpenTodoApp/OpenTodo)
```

## Branches, tags and versions

- `release-X.Y.Z` branches are the shipped lines; the current one is `release-1.7.0`.
  `main` is the older development line (it still carries the 1.4.2-era history and does
  not contain the 1.5-onward release lines; see the cutover below).
- Every synced upstream release is marked with a local tag **`upstream-vX.Y.Z`**
  (same commit as upstream's `vX.Y.Z`). `git log upstream-vX.Y.Z..HEAD --oneline`
  is this fork's exact delta on top of that release.
- Fork releases are tagged **`opentodo-X.Y.Z-N`** (for example `opentodo-1.5.0-1`)
  and published by CI (binaries, GitHub release). The in-app version string comes from
  the build's `VERSION` argument, so a server built from `release-1.7.0` reads `v1.7.0`.
- Feature work happens on branches off the current release line; merge back with a PR.

## When upstream re-creates its history (the 1.5.0 cutover)

Upstream re-created its repository for 1.5.0: the new `v1.5.0` tag has a different root
commit from `v1.4.2`, so `git merge v1.5.0` fails with **"refusing to merge unrelated
histories"** and a forced merge would conflict across the whole tree. Releasing 1.5.0
therefore took a one-time re-base instead of a merge:

```sh
git fetch upstream --tags
git checkout -b release-1.5.0 v1.5.0
# Replay only the paths this fork owns, from the upstream tag it was last built on
# to its previous release branch.
git diff v1.4.2 release-1.4.2 -- \
  web/src/app web/src/lib web/src/index.css web/src/globals.d.ts \
  web/vite.config.ts internal/sync/ops.go | git apply -3
```

Resolve the conflicts by keeping this fork's change where it still applies and upstream's
side where it does not, then finish the release:

1. **Locales:** do not re-extract the catalogs from the new tag (the extractor's key set
   moves with each release). Keep upstream's `web/src/locales/*.json` and append only the
   fork's new strings, in upstream's ordering.
2. **Op schema:** `UPDATE_OP_SCHEMA=1 go test ./internal/sync -run TestGeneratedOpSchema`
   after `internal/sync/ops.go` changed, and commit the regenerated
   `web/src/generated/opSchema.ts` and `internal/sync/testdata/schema_fixture.json`.
3. **Checks:** `go build ./cmd/... ./internal/...` (not `./...` with `web/node_modules`
   present), `go test ./internal/sync`, and in `web/`: `npx tsc --noEmit`,
   `npx tsc -p tsconfig.sw.json`, `npm run lint`, plus the affected vitest suites.
4. **Adopt the line:** once it is green, point `main` at it and push both:
   `git branch -f main release-1.5.0 && git checkout main`, then
   `git push --force-with-lease fork main release-1.5.0`.

History is continuous again from `v1.5.0`: it is an ancestor of upstream's `main` and of
`release-1.5.0`, so every later release merges normally. Treat this section as the
recipe for a future re-creation, not as the routine path.

## Syncing an upstream release

Either run the weekly workflow (opens a "Sync upstream vX.Y.Z" PR, or an issue when the
release needs a manual re-base), or by hand:

```sh
git fetch upstream --tags
scripts/sync-upstream.sh vX.Y.Z              # merge, or re-base on unrelated history
scripts/sync-upstream.sh vX.Y.Z release-1.5.0  # explicit fork side for a re-base
```

The script refuses to run with a dirty tree and tags `upstream-vX.Y.Z`. When the tag
shares history with the fork it checks out `main` and merges with `--no-ff`, as before.
When it does not (upstream re-created the repository), it creates `release-X.Y.Z` at the
tag and replays the fork's delta over the paths this fork owns, then prints the locale,
schema and check steps above. Conflicts are left in the tree for you to resolve; nothing
is pushed.

Resolve conflicts by taking upstream's side where this fork's change no longer applies,
and re-applying this fork's change on top where it still matters.

## What a sync usually conflicts on

The fork speaks upstream's name everywhere, so there is no brand sweep to re-apply. What
still diverges from upstream and needs a hand during a merge:

| Area | Files |
| --- | --- |
| This fork's features | the global motion switch (`web/src/lib/rootMotion.ts`, `appearance.ts`, `AppearanceSection.tsx`), the grip hold-to-undo and its hint (`GripHint.tsx`, `useListDrag.ts`, `dnd.ts`), the About version row and changelog URL (`AboutSection.tsx`) |
| Removed features | the built-in plugins settings section (deleted here; a sync that brings it back is re-removed) |
| Distribution tooling | `FORKING.md`, `README.md`, `scripts/sync-upstream.sh`, `scripts/upstream-pr.sh`, `.github/workflows/sync-upstream.yml` |
| Generated and translated files | `web/src/locales/*.json`, `web/src/generated/opSchema.ts`, `internal/sync/testdata/schema_fixture.json`, `CHANGELOG.md` |

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
the PR against `OpenTodoApp/OpenTodo`. Run it with a clean tree; on a cherry-pick conflict
it stops and prints the manual steps. Follow upstream's CONTRIBUTING: bug fixes need no
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
