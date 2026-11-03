// Compiles CHANGELOG.md into web/src/generated/changelog.json (OpenSpec change
// version-and-grip-hint): Settings -> About opens the release notes from this lazy
// module, so the dialog ships the repository's own changelog without the network.
// Keep a Changelog headings become releases (`## [0.19.0] - 2026-10-31`), `###`
// sections become the release's groups and `-` bullets become its items; a bullet's
// wrapped continuation lines join it with a space, and the prose keeps its inline
// **bold**, `code` and [links](url) for the dialog to render.
//
//   node scripts/changelog-build.mjs           writes web/src/generated/changelog.json
//   node scripts/changelog-build.mjs --check   fails when the committed file is not current
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

export const OUT = 'web/src/generated/changelog.json'
const repo = join(dirname(fileURLToPath(import.meta.url)), '..')

/** CHANGELOG.md's text -> { releases: [{ version, date?, sections: [{ kind, items }] }] }. */
export function parseChangelog(md) {
  const releases = []
  let release = null
  let section = null
  let bullet = null
  const flushBullet = () => {
    if (bullet && section) section.items.push(bullet.text.trim())
    bullet = null
  }
  const flushSection = () => {
    flushBullet()
    if (release && section && section.items.length) release.sections.push(section)
    section = null
  }
  const flushRelease = () => {
    flushSection()
    if (release && release.sections.length) releases.push(release)
    release = null
  }
  for (const line of md.split('\n')) {
    const head = /^## \[([^\]]+)\](?: - (\d{4}-\d{2}-\d{2}))?$/.exec(line)
    if (head) {
      flushRelease()
      release = { version: head[1], ...(head[2] ? { date: head[2] } : {}), sections: [] }
      continue
    }
    if (!release) continue // the file's introduction, never shown
    const kind = /^### (.+)$/.exec(line)
    if (kind) { flushSection(); section = { kind: kind[1].trim(), items: [] }; continue }
    if (!section) continue
    if (bullet && /^\s+\S/.test(line)) { bullet.text += ` ${line.trim()}`; continue } // a wrapped bullet
    flushBullet()
    if (line.startsWith('- ')) bullet = { text: line.slice(2) }
  }
  flushRelease()
  return { releases }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const out = parseChangelog(readFileSync(join(repo, 'CHANGELOG.md'), 'utf8'))
  const text = `${JSON.stringify(out, null, 2)}\n`
  const path = join(repo, OUT)
  if (process.argv.includes('--check')) {
    const current = existsSync(path) ? readFileSync(path, 'utf8') : ''
    if (current !== text) {
      console.error(`changelog-build: ${OUT} is not current; run node scripts/changelog-build.mjs`)
      process.exit(1)
    }
    console.log(`changelog-build: ok (${out.releases.length} releases)`)
  } else {
    writeFileSync(path, text)
    console.log(`changelog-build: wrote ${out.releases.length} releases to ${OUT}`)
  }
}
