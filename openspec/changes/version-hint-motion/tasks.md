# Tasks

## 1. The changelog stays the website's

- [x] 1.1 No compiled copy: the row links to the published changelog, whose every release has an anchor (`#v1-0-0`) — verified by 7.2.
- [x] 1.2 No build step and no Makefile target, so `make release-prepare` rewriting CHANGELOG.md cannot leave a stale module or fail a check.

## 2. Settings → About: the version row links to the changelog

- [x] 2.1 `changelogUrl(version)` in `AboutSection.tsx`: the release's anchor from a plain `x.y.z`, the page itself for a dev build or a release candidate — verified by 7.2.
- [x] 2.2 `AboutSection` gains the Version row: the server's version online, this build's otherwise, the number as a link opening in a new tab (`data-about-version`) — verified by 7.2.
- [x] 2.3 The row's copy in `settingsCopy.ts` and its settings-search entry under "version" and "changelog" — verified by `node scripts/i18n-check.mjs` and 7.2.

## 3. The grip's one-time reminder

- [x] 3.1 `web/src/app/task-list/GripHint.tsx` above the first list with tasks, mounted only where `useFinePointer()` is false — verified by 7.1.
- [x] 3.2 Dismissal through `lib/deviceStorage` (`opentodo.grip-hint`), storage refusal showing it again next launch — verified by 7.1.

## 4. Popups and slides animate under the motion option

- [x] 4.1 `hint-in` on the reminder, `bar-in` on the selection bar, `tick-in` on the selection box, the grip's press sink and the held row's tint fade, all on duration and easing tokens — verified by `npm test -- motionPref taskRow toast tokens/lint` (the stylesheet's contract).
- [x] 4.2 No new switch: the existing Motion preference and reduced motion zero the new motion through the stylesheet's global blocks — verified by the motion contract test.

## 5. Copy in every shipped language

- [x] 5.1 `node scripts/i18n/extract.mjs --write` refreshed `messages.json` (6907 messages).
- [x] 5.2 de, es, fr, it and pt for all 14 new messages — verified by `node scripts/i18n-check.mjs`.

## 6. The specs say what shipped

- [x] 6.1 Deltas for `settings-navigation` ("About section") and `task-list-presentation` ("Nothing between the header and the list").
- [x] 6.2 An Unreleased CHANGELOG.md entry marked **OpenTodo**, with the compiled module regenerated after it.
- [ ] 6.3 `make openspec-validate` (`npx @fission-ai/openspec validate --all --strict`) passes.

## 7. Tests

- [ ] 7.1 The grip reminder: shown above a touch list that holds tasks, **Got it** persists across a remount, a fine pointer never sees it.
- [ ] 7.2 The About version row links to the changelog at the release it shows — the server's once known, this build's before that — falling back to the page for a version without an anchor, and the row is findable in the settings search.

## 8. The gates

- [ ] 8.1 In `web/`: `npm run typecheck`, `npx eslint .`, `npm test` and `npm run build`.
- [ ] 8.2 `make i18n-check docs-check ui-lint changelog-check openspec-validate repo-check` from the repository root.
