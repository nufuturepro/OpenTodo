// The changelog parser (OpenSpec change version-and-grip-hint): releases, dates,
// sections, wrapped bullets, and the repository's own CHANGELOG.md end to end.
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { parseChangelog } from './changelog-build.mjs'

const MD = `# Changelog

The introduction nobody shows.

## [Unreleased]

### Added

- **A thing.** With **bold** and a [link](https://example.com).

## [0.9.0] - 2026-01-02

### Fixed

- One line.
- A bullet that
  wraps onto the next line.

### Security

- **Hard.** Done.
`

test('headings become releases, sections and joined bullet items', () => {
  const out = parseChangelog(MD)
  assert.deepEqual(out.releases.map((r) => r.version), ['Unreleased', '0.9.0'])
  assert.equal(out.releases[0].date, undefined)
  assert.equal(out.releases[1].date, '2026-01-02')
  assert.deepEqual(out.releases[1].sections.map((s) => s.kind), ['Fixed', 'Security'])
  assert.deepEqual(out.releases[1].sections[0].items, ['One line.', 'A bullet that wraps onto the next line.'])
  assert.ok(out.releases[0].sections[0].items[0].includes('[link](https://example.com)'))
})

test('the repository CHANGELOG.md parses into releases with items', () => {
  const md = readFileSync(new URL('../CHANGELOG.md', import.meta.url), 'utf8')
  const out = parseChangelog(md)
  assert.equal(out.releases[0].version, 'Unreleased')
  assert.ok(out.releases.length > 10, `only ${out.releases.length} releases`)
  const first = out.releases.find((r) => r.version !== 'Unreleased')
  assert.ok(first.date, 'a release carries its date')
  assert.ok(first.sections.length > 0 && first.sections[0].items.length > 0)
})
