// @vitest-environment happy-dom
// Settings → About's Version row (OpenSpec change version-hint-motion, settings-navigation
// "About section"): the number itself opens this repository's changelog from the compiled
// module — offline, newest release first — and the settings search finds the row.
import 'fake-indexeddb/auto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { version as buildVersion } from '../../package.json'
import type { User } from '../lib/types'
import { click, render, waitFor } from '../ui/test-utils'
import { AboutSection } from './AboutSection'
import { searchSettings } from './settingsIndex'

const member: User = { id: 'u1', email: 'sam@example.com', name: 'Sam', role: 'member', createdAt: 1 }

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 404 })))
  localStorage.clear()
})
afterEach(() => {
  document.body.innerHTML = ''
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('the Version row', () => {
  it('shows this build’s version while the server’s is unknown, and opens the changelog', async () => {
    const r = await render(<AboutSection user={member} />)
    const row = document.querySelector<HTMLElement>('[data-about-version]')!
    expect(row.textContent).toBe(buildVersion)
    await click(row)
    await waitFor(() => !!document.querySelector('[data-changelog]'), 'the changelog to load')
    const dialog = document.querySelector('[data-testid="changelog"]')!
    expect(dialog.getAttribute('aria-labelledby') || dialog.textContent).toBeTruthy()
    // The newest release of this repository's own changelog, with its section headings.
    expect(document.querySelector('[data-changelog-version]')!.textContent).toContain('Unreleased')
    expect(dialog.textContent).toContain('The version opens the changelog.')
    expect(dialog.textContent).toContain('Added')
    expect(dialog.textContent).toContain('Security')
    await r.unmount()
  })

  it('is found by the settings search under "version" and "changelog"', () => {
    for (const q of ['version', 'changelog']) {
      const entries = searchSettings(q, member).flatMap((hit) => hit.entries)
      expect(entries.some((e) => e.anchor === 'about-version' && e.label === 'Version')).toBe(true)
    }
  })
})
