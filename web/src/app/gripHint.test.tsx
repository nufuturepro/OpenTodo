// @vitest-environment happy-dom
// The grip's one-time reminder (OpenSpec change version-hint-motion, task-list-presentation
// "Nothing between the header and the list"): above the first touch list that holds tasks,
// dismissed once per device through deviceStorage, never where the pointer is fine.
import 'fake-indexeddb/auto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useDatabase } from '../lib/db'
import { createTask, resetDeviceId } from '../lib/ops'
import { minWidthPx, render, waitFor, click } from '../ui/test-utils'
import { resetPanel } from './panel'
import { TaskListView } from './TaskListView'
import { gripHintDismissed } from './task-list/GripHint'

const original = window.matchMedia
let fine = false
let reduce = false
let n = 0
beforeEach(() => {
  useDatabase(`griphint-${Date.now()}-${n++}`)
  resetDeviceId()
  localStorage.clear()
  fine = false
  reduce = false
  // By default a phone: a coarse pointer (a fine one is set per test).
  window.matchMedia = ((q: string) => ({
    matches: /pointer:\s*fine/.test(q) ? fine : /pointer:\s*coarse/.test(q) ? !fine : /prefers-reduced-motion/.test(q) ? reduce : minWidthPx(q) <= 390,
    media: q, onchange: null, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, dispatchEvent: () => false,
  })) as typeof window.matchMedia
})
const mounted: { unmount: () => Promise<void> }[] = []
afterEach(async () => {
  for (const r of mounted.splice(0)) await r.unmount()
  window.matchMedia = original
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  resetPanel()
})

/** The inbox with these tasks, mounted and settled. */
async function inbox(...titles: string[]) {
  for (const t of titles) await createTask({ title: t })
  const r = await render(<TaskListView route={{ view: 'inbox' }} />)
  mounted.push(r)
  await waitFor(() => document.body.textContent!.includes(titles[titles.length - 1]))
  return r
}

describe('the grip reminder', () => {
  it('shows above the first touch list that holds tasks, ahead of the rows', async () => {
    await inbox('Water plants')
    await waitFor(() => !!document.querySelector('[data-grip-hint]'), 'the reminder')
    const hint = document.querySelector('[data-grip-hint]')!
    expect(hint.textContent).toContain('Press and hold a task’s grip to select it.')
    expect(hint.querySelector('[data-grip-hint-dismiss]')?.textContent).toBe('Got it')
    const firstRow = document.querySelector('[data-task-id]')!
    // It sits above the rows, not between or below them.
    expect(!!(hint.compareDocumentPosition(firstRow) & Node.DOCUMENT_POSITION_FOLLOWING)).toBe(true)
  })

  it('Got it dismisses it for this device, and a remount keeps it dismissed', async () => {
    await inbox('Water plants')
    await waitFor(() => !!document.querySelector('[data-grip-hint]'), 'the reminder')
    await click(document.querySelector<HTMLButtonElement>('[data-grip-hint-dismiss]')!)
    await waitFor(() => !document.querySelector('[data-grip-hint]'), 'the reminder gone')
    expect(localStorage.getItem('opentodo.grip-hint')).toBe('1')
    expect(gripHintDismissed()).toBe(true)
    // A fresh mount of the list — the next launch — does not bring it back.
    const again = await render(<TaskListView route={{ view: 'inbox' }} />)
    mounted.push(again)
    await waitFor(() => document.body.textContent!.includes('Water plants'))
    expect(document.querySelector('[data-grip-hint]')).toBeNull()
  })

  it('waits until the list holds a task', async () => {
    const r = await render(<TaskListView route={{ view: 'inbox' }} />)
    mounted.push(r)
    await waitFor(() => document.body.textContent!.includes('Inbox'), 'the list')
    expect(document.querySelector('[data-grip-hint]')).toBeNull()
  })

  it('never shows where the pointer is fine', async () => {
    fine = true
    await inbox('Water plants')
    expect(document.querySelector('[data-grip-hint]')).toBeNull()
  })
})
