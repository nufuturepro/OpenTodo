# Design

## D1 — The changelog stays the website's

The published changelog renders every release with its summary, its section headings and its own resolution of the `docs/…` links that CHANGELOG.md writes relative to the repository; the app has none of that, and a copy of it would drift the moment a release is cut — `make release-prepare` rewrites CHANGELOG.md, so a compiled copy is stale after every release. So the row links out: `changelogUrl()` builds `https://open-todo.app/changelog/#v1-0-0` from the version it shows (dots to dashes, the anchor the site publishes for each release). A build without a release number or a release candidate has no anchor and opens the page's newest release, which is the release it is closest to. The cost is a document that used to open offline: it now needs a connection, like every other link on the page, and in exchange nothing is generated, committed, checked on every release or paid for in the bundle.

## D2 — Version: the server's word, the build's fallback

The row shows `useServerVersion()`'s value — the same one the About card's description already prints while online — and falls back to `web/package.json`'s version offline, so the number is never absent and the link always has an anchor to build. One row, one control: the number itself is the link. Nothing new is fetched; the row reuses the About section's existing status call.

## D3 — The reminder is device state

`opentodo.grip-hint` through `lib/deviceStorage`, exactly the swipe cue's pattern: read at mount, written on **Got it**, and a refusal of storage only means the reminder shows again next launch. Coarse pointers only, gated at the mount point with the list's existing `useFinePointer()`, and only a list that holds tasks — an empty first-run Inbox shows its start guide instead, and the reminder appears once there is something to grip. **Data model impact: none.** No `user_prefs` field, no op, nothing to converge — the dismissal has no meaning beyond this device.

## D4 — Motion rides the option that already exists

Every added entrance and transition uses the design system's tokens (`--duration-ui`, `--duration-emphasis`, `--ease-calm`, `--ease-spring`). The stylesheet's `:root[data-motion="none"]` block already zeroes animation and transition durations for every element and the `prefers-reduced-motion` block already limits transitions to colour and opacity, so Settings → Appearance's Motion switch and the system preference both cover the new motion with no new option and no per-surface guards — that is the requested reduce-or-turn-off control, and the reminder, bar, box, grip and tint all obey it. Reduced motion keeps the tint and press colours as fades and drops the movement, which matches design-system "Reduced motion".

## Interface budget (docs/interface-guide.md)

The list page today holds the page header and the quick add above its first task. The reminder adds one row, so while it is shown the page holds two rows above the first task — the stated maximum — and it is one-time, touch-only and leaves permanently on dismiss; it is content the gesture needs at the place the hand is, which is why it is not folded into the header. The Version row is a settings row, where the budget does not apply. **No new shortcut** is registered, so the registry has no conflict to resolve.

## Dependencies, image size and memory

No new dependency: the parser is plain Node (the repository already ships such scripts), the compiled changelog is a lazy chunk of a few dozen KiB next to `templates.json` and `help.json`, and the dialog holds its content only while open. Container image size and runtime memory are unchanged — nothing was added to the server.
