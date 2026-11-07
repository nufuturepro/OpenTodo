# The version, its changelog and the grip's reminder

## Why

The version today lives only in the About card's description sentence, only while the server is online, so a reader who wants to know what they run — or what changed in it — has nowhere to look, and this repository's own CHANGELOG.md is readable only in the repository. The grip that `touch-row-grip` gave to touch selection is invisible in the same way: the selection bar and the header's Select print their words, but the gesture has no on-screen lesson, and the swipe cue's toast does not name it. And the surfaces this change adds — a card above a list, a selection bar — must enter the way the design system's motion says popups and slides enter, under the motion option the account already has, instead of jumping into place.

## What Changes

- **A Version row in Settings → About opens the changelog.** The row's control is the version number itself — the server's while online, this build's otherwise — and it is a link to the published changelog on `open-todo.app`, at the release the client runs (the page anchors every release, `#v1-0-0`), with the whole history below it. The website already renders the summaries, the section headings and the `docs/…` links the app has no copy of, so the app carries no changelog of its own: nothing is generated, nothing is committed and no release has to rebuild anything. A build without a release number (`dev+abc1234`) or a release candidate opens the page's newest release. The settings search finds the row under "version" and "changelog".
- **A one-time reminder teaches the grip above the first list that holds tasks.** On a device whose primary pointer is coarse, one card — "Press and hold a task's grip to select it." with **Got it** — sits above the rows until that device dismisses it, then never returns on that device. It follows the swipe cue's pattern: device-local storage, no sync, no operation.
- **The new popups and slides animate with the shared motion.** The reminder slides in from under the page header, the selection bar rises with the first tick, the selection box pops in where the completion circle stood, the grip sinks under a press and the held row's tint fades rather than flips — all on the design system's duration and easing tokens. The account's existing Motion switch (Settings → Appearance) and the system's reduced motion zero all of it, so the option to reduce or turn off animation is the same one the app already offers; this change adds no second switch.

No new environment variable and **no breaking change to the op log format**: the reminder's dismissal is device-local storage, the changelog stays the website's, and selection stays device state.

## Capabilities

### Modified Capabilities

- `settings-navigation`: the About section gains a Version row — the server's version while online, this build's otherwise — that links to the published changelog at that release, found by the settings search under "version" and "changelog".
- `task-list-presentation`: "Nothing between the header and the list" gains one named exception — the grip's one-time, per-device reminder card, one row, touch only, gone forever once dismissed — so the page stays within two rows of chrome above the first task while it is shown.

## Impact

- Web app: `web/src/app/AboutSection.tsx` (the Version row, linking to the changelog at `changelogUrl()`), `web/src/app/task-list/GripHint.tsx` (new) and its mount in `web/src/app/TaskListView.tsx`, `web/src/index.css` (the reminder, bar, tick, grip and tint motion).
- Copy: `web/src/lib/settingsCopy.ts`, `web/src/app/settingsIndex.ts`, and `web/src/locales/messages.json` plus the de, es, fr, it and pt catalogs.
- Tests: a grip-reminder test and the About Version row test, anchoring a release and falling back to the newest one (tasks 7.1–7.2).
- Docs: one Unreleased CHANGELOG.md entry marked **OpenTodo**; `docs/` stays as it is, since the gesture is already documented in `docs/work-from-the-list.md` and the About section in the settings guide.
- Out of scope: syncing the reminder across devices (no `user_prefs` field), a server-reported changelog, changes to the start guide or the swipe cue's toast, new shortcuts, and any server, API or sync change.
- Advances no ROADMAP.md item: the original roadmap is complete, so this is follow-up polish on shipped capabilities — About (`settings-navigation`), touch selection (`bulk-actions` through `touch-row-grip`) and the motion system (`design-system`).
