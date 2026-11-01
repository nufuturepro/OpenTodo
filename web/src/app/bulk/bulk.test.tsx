// @vitest-environment happy-dom
// Bulk selection, bar, Actions menu and undo toast in a list (OpenSpec change
// bulk-edit, tasks 2.1, 2.3, 2.4, 2.5, 3.3, 4.2 and 5.1).
import 'fake-indexeddb/auto'
import { act } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { db, useDatabase } from '../../lib/db'
import { setDateTime } from '../../lib/dates'
import { createLabel } from '../../lib/labels'
import { createProject, createTask, resetDeviceId, updateTask } from '../../lib/ops'
import { key, render, waitFor } from '../../ui/test-utils'
import { PanelHost } from '../PanelHost'
import { resetPanel } from '../panel'
import { TaskListView } from '../TaskListView'
import { hideBulkToast } from './toastStore'
import { UndoToast } from './UndoToast'

let n = 0
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 9, 14, 12, 0))
  useDatabase(`bulk-ui-${Date.now()}-${n++}`)
  resetDeviceId()
  setDateTime({ date_format: 'day-month', time_format: '24h', week_start: 'mon' }, { order: 'MD', hour12: true, weekStart: 1 })
})
const mounted: { unmount: () => Promise<void> }[] = []
afterEach(async () => {
  for (const r of mounted.splice(0)) await r.unmount()
  vi.useRealTimers()
  hideBulkToast()
  resetPanel()
  // A case that opened a task page leaves its address; the next list starts from the Inbox's.
  history.replaceState(null, '', '/')
})

const tickedTitles = () => [...document.querySelectorAll('[data-ticked] [data-tick]')].map((e) => e.getAttribute('aria-label')!.replace('Select ', ''))
const option = (name: string) => [...document.querySelectorAll('[data-picker-step] [role="option"]')].find((o) => o.textContent?.includes(name)) as HTMLElement | undefined
const errandOption = () => option('errand')
const bar = () => document.querySelector('[data-selection-bar] [role="status"]')?.textContent ?? null

async function inbox(...titles: string[]) {
  const ids: string[] = []
  for (const t of titles) ids.push(await createTask({ title: t }))
  const r = await render(<><TaskListView route={{ view: 'inbox' }} /><PanelHost /><UndoToast /></>)
  mounted.push(r)
  await waitFor(() => document.body.textContent!.includes(titles[titles.length - 1]))
  return { r, ids }
}

describe('ticking from the keyboard (2.1, 2.4)', () => {
  it('x then Shift+ArrowDown twice ticks three rows; the bar counts them; Escape clears', async () => {
    await inbox('One', 'Two', 'Three', 'Four')
    await key(document.body, 'j')
    await key(document.body, 'x')
    await waitFor(() => bar() === '1 selected')
    await key(document.body, 'ArrowDown', { shiftKey: true })
    await key(document.body, 'ArrowDown', { shiftKey: true })
    await waitFor(() => bar() === '3 selected')
    expect(tickedTitles()).toEqual(['One', 'Two', 'Three'])
    await key(document.body, 'Escape')
    await waitFor(() => bar() === null)
  })

  it('Space with no cursor and Ctrl+A with nothing ticked fall through to the browser', async () => {
    await inbox('One', 'Two')
    const space = new KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true })
    await act(async () => { document.body.dispatchEvent(space) })
    expect(space.defaultPrevented).toBe(false)
    const all = new KeyboardEvent('keydown', { key: 'a', ctrlKey: true, bubbles: true, cancelable: true })
    await act(async () => { document.body.dispatchEvent(all) })
    expect(all.defaultPrevented).toBe(false)
    await key(document.body, 'j')
    await key(document.body, ' ')
    await key(document.body, 'a', { ctrlKey: true })
    await waitFor(() => bar() === '2 selected')
  })

  it('Escape clears the ticks; Enter opens the selected task as its page (task-full-page)', async () => {
    const { ids } = await inbox('One', 'Two')
    await key(document.body, 'j')
    await key(document.body, 'x')
    expect(bar()).toBe('1 selected')
    await key(document.body, 'Escape')
    await waitFor(() => bar() === null)
    await key(document.body, 'Enter')
    // On the Inbox's split the task stood beside the list until Enter opened its page (inbox-split D1).
    await waitFor(() => ids.some((id) => location.pathname === `/task/${id}`))
  })
})

describe('ticking with the pointer (2.3)', () => {
  it('Shift+click ticks a range and opens nothing; the checkbox passes axe', async () => {
    await inbox('One', 'Two', 'Three')
    const box = (title: string) => document.querySelector(`[data-tick][aria-label="Select ${title}"]`) as HTMLInputElement
    await act(async () => { box('One').dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true })) })
    await act(async () => { box('Three').dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, shiftKey: true })) })
    await waitFor(() => bar() === '3 selected')
    expect(document.querySelector('aside[data-detail-for]')).toBeNull()
    const axe = (await import('axe-core')).default
    const res = await axe.run(document.querySelector('ul, [role="list"]') ?? document.body, { rules: { 'color-contrast': { enabled: false } } })
    expect(res.violations.filter((v) => v.id !== 'region').map((v) => v.id)).toEqual([])
  })
})

describe('actions (2.5, 3.3, 4.2)', () => {
  it('c on three ticked rows completes them in one batch, and Undo restores them', async () => {
    const { ids } = await inbox('One', 'Two', 'Three')
    await key(document.body, 'j'); await key(document.body, 'x')
    await key(document.body, 'ArrowDown', { shiftKey: true }); await key(document.body, 'ArrowDown', { shiftKey: true })
    await db.outbox.clear()
    await key(document.body, 'c')
    await waitFor(() => !!document.querySelector('[data-bulk-toast]'))
    expect(document.querySelector('[data-bulk-toast]')!.textContent).toContain('3 tasks completed')
    expect(new Set((await db.outbox.toArray()).map((o) => o.ts)).size).toBe(1)
    await act(async () => { (document.querySelector('[data-bulk-toast] button') as HTMLButtonElement).click() })
    await waitFor(() => document.querySelector('[data-bulk-toast]')!.textContent!.includes('Undone: 3 restored'))
    for (const id of ids) expect((await db.tasks.get(id))!.completed_at).toBeNull()
  })

  it('`.` with four ticked opens a menu titled "4 tasks"; two projects disable Assign with the reason', async () => {
    const a = await createProject({ name: 'A' })
    await createTask({ title: 'In A', project_id: a })
    await inbox('One', 'Two', 'Three', 'Four')
    await key(document.body, 'j'); await key(document.body, 'x')
    for (let i = 0; i < 3; i++) await key(document.body, 'ArrowDown', { shiftKey: true })
    await key(document.body, '.')
    await waitFor(() => document.querySelectorAll('[role="menuitem"]').length > 0)
    expect(document.querySelector('[aria-label="Actions for 4 tasks"]')!.getAttribute('aria-expanded')).toBe('true')
    const assign = [...document.querySelectorAll('[role="menuitem"]')].find((m) => m.textContent?.includes('Assign'))!
    expect((assign as HTMLButtonElement).disabled).toBe(true)
    expect(document.body.textContent).toContain('Tasks must be in one shared project')
  })

  it('More on a row opens the menu at that button, without the Priority and Date its neighbours offer; `.` keeps them', async () => {
    await inbox('One', 'Two')
    const items = () => [...document.querySelectorAll('[role^="menuitem"]')].map((m) => m.textContent ?? '')
    const more = document.querySelector<HTMLButtonElement>('[data-task-id] [data-row-action="more"]')!
    await act(async () => { more.click() })
    await waitFor(() => items().length > 0)
    expect(items().some((m) => m.includes('Move to project'))).toBe(true)
    expect(items().some((m) => m.startsWith('Date') || m.startsWith('Priority'))).toBe(false)
    // Closing returns focus to the More button the menu was placed at.
    await key(document.activeElement as HTMLElement, 'Escape')
    await waitFor(() => items().length === 0)
    expect(document.activeElement).toBe(more)
    await key(document.body, 'j')
    await key(document.body, '.')
    await waitFor(() => items().length > 0)
    // The keyboard menu keeps both: the priority strip and the date group's own choices.
    expect(items().some((m) => m.startsWith('Pick a date…'))).toBe(true)
    expect([...document.querySelectorAll<HTMLElement>('[role="menuitemcheckbox"]')].filter((b) => /^Priority [1-4]$/.test(b.getAttribute('aria-label') ?? '')).length).toBe(4)
  })

  it('Priority and Date are one item each: their choices open in place, Escape goes back', async () => {
    await inbox('One')
    const items = () => [...document.querySelectorAll<HTMLElement>('[role^="menuitem"]')]
    const item = (label: string) => items().find((m) => m.textContent?.startsWith(label))
    await key(document.body, 'j')
    await key(document.body, '.')
    await waitFor(() => items().length > 0)
    // The choices are open in place from the start: the priority strip, the date row.
    const digits = [...document.querySelectorAll<HTMLElement>('[role="menuitemcheckbox"]')].filter((b) => /^Priority [1-4]$/.test(b.getAttribute('aria-label') ?? ''))
    expect(digits.map((d) => d.textContent)).toEqual(['1', '2', '3', '4'])
    expect(item('Tomorrow')).toBeDefined()
    expect(item('Move to project')).toBeDefined()
    // The date panel extends the menu beside its items; Escape steps back to them.
    await act(async () => { item('Pick a date…')!.click() })
    await waitFor(() => !!document.querySelector('[data-date-panel]'))
    expect(item('Move to project')).toBeDefined()
    await key(document, 'Escape')
    await waitFor(() => !document.querySelector('[data-date-panel]'))
    expect(document.activeElement?.getAttribute('data-menu-label')).toBe('Pick a date…')
    await key(document, 'Escape')
    await waitFor(() => items().length === 0)
  })

  it('Date → Tomorrow reschedules the ticked tasks', async () => {
    const { ids } = await inbox('One', 'Two', 'Three')
    await key(document.body, 'j'); await key(document.body, 'x')
    await key(document.body, 'ArrowDown', { shiftKey: true }); await key(document.body, 'ArrowDown', { shiftKey: true })
    await key(document.body, '.')
    await waitFor(() => document.querySelectorAll('[role="menuitem"]').length > 0)
    await waitFor(() => [...document.querySelectorAll('[role="menuitem"]')].some((m) => m.textContent?.startsWith('Tomorrow')))
    await act(async () => { ([...document.querySelectorAll('[role="menuitem"]')].find((m) => m.textContent?.startsWith('Tomorrow')) as HTMLElement).click() })
    await waitFor(() => !!document.querySelector('[data-bulk-toast]'))
    for (const id of ids) expect((await db.tasks.get(id))!.due).toEqual({ date: '2026-10-15' })
  })

  it('the Priority group is one strip of four digits with full accessible names, the shared one checked', async () => {
    const { ids } = await inbox('One', 'Two')
    await updateTask(ids[0], { priority: 2 })
    await updateTask(ids[1], { priority: 2 })
    await key(document.body, 'j'); await key(document.body, 'x')
    await key(document.body, 'ArrowDown', { shiftKey: true })
    await key(document.body, '.')
    await waitFor(() => document.querySelectorAll('[role="menuitem"]').length > 0)
    const digits = [...document.querySelectorAll('[role="menuitemcheckbox"]')].filter((b) => /^Priority [1-4]$/.test(b.getAttribute('aria-label') ?? ''))
    // Four buttons showing only their digit, all in one row under the group label.
    expect(digits.map((d) => d.textContent)).toEqual(['1', '2', '3', '4'])
    expect(new Set(digits.map((d) => d.parentElement)).size).toBe(1)
    // The shared priority is checked; a diverging task unchecks it live.
    await waitFor(() => digits[1].getAttribute('aria-checked') === 'true')
    expect(digits.map((d) => d.getAttribute('aria-checked'))).toEqual(['false', 'true', 'false', 'false'])
    await updateTask(ids[1], { priority: 4 })
    await waitFor(() => digits.every((d) => d.getAttribute('aria-checked') === 'false'))
    // Type-ahead "pri" still lands on the strip.
    await key(digits[0], 'p'); await key(digits[0], 'r'); await key(digits[0], 'i')
    expect(document.activeElement?.getAttribute('aria-label')).toMatch(/^Priority [1-4]$/)
  })

  it('L then "errand" adds it to the tasks lacking it and the picker stays open', async () => {
    const l = await createLabel({ name: 'errand' })
    const { ids } = await inbox('One', 'Two', 'Three')
    await updateTask(ids[0], { labels: [l] })
    await key(document.body, 'j'); await key(document.body, 'x')
    await key(document.body, 'ArrowDown', { shiftKey: true }); await key(document.body, 'ArrowDown', { shiftKey: true })
    await waitFor(() => bar() === '3 selected')
    await key(document.body, 'L', { shiftKey: true })
    await waitFor(() => !!document.querySelector('[data-picker-step]'))
    // Picked at once, before the menu's own read lands: the click reads the tasks itself.
    await act(async () => { ([...document.querySelectorAll('[data-picker-step] [role="option"]')].find((o) => o.textContent?.includes('errand')) as HTMLElement).click() })
    await waitFor(() => !!document.querySelector('[data-bulk-toast]'), 'the undo offer')
    // The toast can show before the batch's last row is read back; wait for the store.
    const labels = () => Promise.all(ids.map(async (id) => (await db.tasks.get(id))!.labels))
    await waitFor(async () => (await labels()).every((ls) => ls.length === 1), 'the label on every task')
    expect(await labels()).toEqual(ids.map(() => [l]))
    expect(document.querySelector('[data-picker-step]')).not.toBeNull()
    // The open picker follows the stored tasks: the label now on every task is marked.
    await waitFor(() => errandOption()?.getAttribute('aria-selected') === 'true')
  })

  it('a picker opened before the read marks the common label once the read lands (precision-polish-2)', async () => {
    const l = await createLabel({ name: 'errand' })
    await createLabel({ name: 'home' })
    const { ids } = await inbox('One', 'Two')
    for (const id of ids) await updateTask(id, { labels: [l] })
    await key(document.body, 'j'); await key(document.body, 'x')
    await key(document.body, 'ArrowDown', { shiftKey: true })
    await waitFor(() => bar() === '2 selected')
    await key(document.body, 'L', { shiftKey: true })
    await waitFor(() => !!document.querySelector('[data-picker-step]'))
    await waitFor(() => errandOption()?.getAttribute('aria-selected') === 'true')
    expect(option('home')?.getAttribute('aria-selected')).toBe('false')
    // Picking it removes it from both, and the mark goes away in the same open picker.
    await act(async () => { errandOption()!.click() })
    await waitFor(() => errandOption()?.getAttribute('aria-selected') === 'false')
    for (const id of ids) expect((await db.tasks.get(id))!.labels).toEqual([])
    // Let the pick's toast land here, not in the next case.
    await waitFor(() => !!document.querySelector('[data-bulk-toast]'))
  })

  it('L then a label every ticked task has removes it, even when picked at once', async () => {
    const l = await createLabel({ name: 'errand' })
    const { ids } = await inbox('One', 'Two')
    for (const id of ids) await updateTask(id, { labels: [l] })
    await key(document.body, 'j'); await key(document.body, 'x')
    await key(document.body, 'ArrowDown', { shiftKey: true })
    await waitFor(() => bar() === '2 selected')
    await key(document.body, 'L', { shiftKey: true })
    await waitFor(() => !!document.querySelector('[data-picker-step]'))
    await act(async () => { ([...document.querySelectorAll('[data-picker-step] [role="option"]')].find((o) => o.textContent?.includes('errand')) as HTMLElement).click() })
    const labels = () => Promise.all(ids.map(async (id) => (await db.tasks.get(id))!.labels))
    await waitFor(async () => (await labels()).every((ls) => ls.length === 0), 'the label off every task')
    expect(await labels()).toEqual(ids.map(() => []))
    // Let the pick's toast land here: shown after the write, it would otherwise
    // appear in the next case and pass for that case's own offer.
    await waitFor(() => !!document.querySelector('[data-bulk-toast]'), 'the undo offer')
  })

  it('Ctrl+Z undoes outside text fields, never inside a textarea', async () => {
    const { ids } = await inbox('One', 'Two')
    const priority = async () => (await db.tasks.get(ids[0]))!.priority
    await key(document.body, 'j'); await key(document.body, 'x')
    await waitFor(() => bar() === '1 selected', 'one row ticked')
    await key(document.body, '1')
    // This case's own offer and write, not a toast an earlier case left behind.
    await waitFor(async () => (await priority()) === 1, 'priority 1 stored')
    await waitFor(() => !!document.querySelector('[data-bulk-toast]')?.textContent?.includes('Priority'), 'the undo offer for the priority')
    const ta = document.createElement('textarea')
    document.body.append(ta)
    const inField = new KeyboardEvent('keydown', { key: 'z', ctrlKey: true, bubbles: true, cancelable: true })
    await act(async () => { ta.dispatchEvent(inField) })
    expect(inField.defaultPrevented).toBe(false)
    expect(await priority()).toBe(1)
    await key(document.body, 'z', { ctrlKey: true })
    await waitFor(() => document.querySelector('[data-bulk-toast]')!.textContent!.includes('Undone'), 'Undone in the toast')
    await waitFor(async () => (await priority()) === 4, 'priority 4 back')
  })
})

// The touch grip: choosing rows from the marker at the row's right edge, while the
// row body's own hold stays the actions menu's gesture (OpenSpec change touch-row-grip).
describe('the touch grip (bulk-actions on touch)', () => {
  // The row menu's own trigger is fixed-positioned at the gesture; the Selection bar
  // has its own unpositioned "Actions for…" button, which this must not confuse.
  const rowMenuOpen = () => [...document.querySelectorAll<HTMLButtonElement>('button[aria-label^="Actions for"]')].some((b) => b.style.position === 'fixed')

  it('a tap on the grip ticks the row, shows every checkbox and the bar, with one haptic', async () => {
    const vibrate = vi.fn()
    Object.defineProperty(navigator, 'vibrate', { value: vibrate, configurable: true })
    const { ids } = await inbox('Tap me')
    const grip = document.querySelector<HTMLElement>(`[data-task-id="${ids[0]}"] [data-row-grip] button`)!
    expect(grip).toBeTruthy()
    await act(async () => { grip.click() })
    await waitFor(() => bar() === '1 selected')
    expect(tickedTitles()).toEqual(['Tap me'])
    expect(vibrate).toHaveBeenCalledWith(10)
    expect(rowMenuOpen()).toBe(false)
  })

  it("a hold on the grip ticks the row (idempotent) and opens no menu", async () => {
    const vibrate = vi.fn()
    Object.defineProperty(navigator, 'vibrate', { value: vibrate, configurable: true })
    const { ids } = await inbox('Hold me')
    const grip = document.querySelector<HTMLElement>(`[data-task-id="${ids[0]}"] [data-row-grip] button`)!
    vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
    try {
      await act(async () => { grip.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, pointerId: 6, pointerType: 'touch', clientX: 20, clientY: 20 })) })
      await act(async () => { vi.advanceTimersByTime(400) })
      await act(async () => { window.dispatchEvent(new PointerEvent('pointerup', { pointerId: 6, pointerType: 'touch', clientX: 20, clientY: 20 })) })
      // The swallowed click's guard ends on a timer; let it run before real timers return.
      await act(async () => { vi.advanceTimersByTime(10) })
      expect(tickedTitles()).toEqual(['Hold me'])
      expect(bar()).toBe('1 selected')
      expect(vibrate).toHaveBeenCalledWith(10)
      expect(rowMenuOpen()).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })

  it("a hold on the row's body arms and opens its menu — it never ticks", async () => {
    const { ids } = await inbox('Body hold')
    const li = document.querySelector<HTMLElement>(`[data-task-id="${ids[0]}"]`)!
    vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
    try {
      await act(async () => { li.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, pointerId: 7, pointerType: 'touch', clientX: 30, clientY: 60 })) })
      await act(async () => { vi.advanceTimersByTime(400) })
      // The hold arms (its tint); selection is the grip's, so nothing ticks.
      expect(li.hasAttribute('data-touch-hold')).toBe(true)
      await act(async () => { window.dispatchEvent(new PointerEvent('pointerup', { pointerId: 7, pointerType: 'touch', clientX: 30, clientY: 60 })) })
      await act(async () => { vi.advanceTimersByTime(10) })
      expect(li.hasAttribute('data-ticked')).toBeFalsy()
      expect(bar()).toBeNull()
      expect(rowMenuOpen()).toBe(true)
    } finally {
      vi.useRealTimers()
    }
  })

  it('a hold that becomes a drag gives its tick back and leaves no bar', async () => {
    const { ids } = await inbox('Drag me', 'Stay put')
    const grip = document.querySelector<HTMLElement>(`[data-task-id="${ids[0]}"] [data-row-grip] button`)!
    vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
    try {
      await act(async () => { grip.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, pointerId: 8, pointerType: 'touch', clientX: 20, clientY: 20 })) })
      await act(async () => { vi.advanceTimersByTime(400) })
      // The hold selected: the row is ticked and the bar says so.
      expect(tickedTitles()).toEqual(['Drag me'])
      expect(bar()).toBe('1 selected')
      // The press becomes a drag: the tick its own hold created is given back at once.
      await act(async () => { window.dispatchEvent(new PointerEvent('pointermove', { pointerId: 8, pointerType: 'touch', clientX: 20, clientY: 200 })) })
      expect(tickedTitles()).toEqual([])
      expect(bar()).toBeNull()
      await act(async () => { window.dispatchEvent(new PointerEvent('pointerup', { pointerId: 8, pointerType: 'touch', clientX: 20, clientY: 200 })) })
      await act(async () => { vi.advanceTimersByTime(10) })
      expect(tickedTitles()).toEqual([])
      expect(bar()).toBeNull()
      expect(rowMenuOpen()).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })
})
