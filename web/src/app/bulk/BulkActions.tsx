// The selection's Actions menu and bar (OpenSpec change bulk-edit, design D3): one
// menu for the ticked tasks (or the cursor task when nothing is ticked), built on
// the shared ActionMenu so its items, pickers, shortcuts and disabled reasons work
// like every other object menu. Moves and assignments go through the review (D6).
import { keysOf } from '../../lib/shortcuts'
import { Kbd } from '../shortcuts/Kbd'
import { useEffect, useMemo, useRef, useState, type RefObject } from 'react'
import { useBelowStickyHeader } from './useBelowStickyHeader'
import { allCompleted, applyBulk, commonSharedProject, readTicked, reviewAssign, reviewMove, type BulkAction, type ReviewPlan, useTickedTasks } from '../../lib/bulk'
import { useAllLabels, useLiveSections, useProjects, useMembershipMap, useReviewRow, useUserPrefsRow } from '../../lib/localData'
import { DAY_PARTS, dayPartName, dayPartOf, dayPartsOn, type DayPart } from '../../lib/dayParts'
import { dayPartUndoable, energyUndoable } from '../../lib/undoActions'
import { ENERGY_LEVELS, energyLevelsOn, energyName, energyOf, type Energy } from '../../lib/energy'
import { itemKeywords, projectSource, sectionSource, type PickerItem } from '../../lib/pickerSources'
import { roleOf } from '../../lib/sharing'
import { toggleTracking, useRunningEntry } from '../../lib/timeEntries'
import { act } from '../actionError'
import type { Label, Membership, Priority, Project, Section, Task } from '../../lib/types'
import { Icon, Input, useFinePointer } from '../../ui'
import { Tooltip } from '../../ui'
import { CalendarGrid, inputLanguages } from '../datetime/DateField'
import { parseDateInput } from '../datetime/parse'
import { formatDate, logicalToday, type ResolvedDateTime } from '../../lib/dates'
import { useDateTimeFormat } from '../DateTimePrefsProvider'
import { ActionMenu, type ActionGroup, type ActionItem } from '../menus/ActionMenu'
import { openRowSchedule } from '../task-list/RowSwipe'
import { BulkReview } from './BulkReview'
import { showBulkToast, showUndoToast } from './toastStore'
import { startFocusOn } from '../focus/startFocus'
import { t } from '../../lib/i18n'
import { withListTransition } from '../motion'
import { slotAction, slotLabel, useSlotState, type SlotState } from '../task-list/slotActions'
import { focusMenuLabel, focusMenuState, runFocusPress, type FocusMenuState } from '../focus/focusRequest'
import { useToday } from '../useViews'

/** Apply a bulk action and offer its undo. */
export async function runBulk(ids: readonly string[], action: BulkAction): Promise<void> {
  if (ids.length === 0) return
  // The rows that stay slide into place; more than the cap update at once (motion-v2 D4).
  const r = await withListTransition(ids.length, () => applyBulk(ids, action))
  showBulkToast(r.label, r.undoId)
}

const tasksWord = (n: number) => t('{n, plural, one {# task} other {# tasks}}', { n })
const itemKey = (i: PickerItem) => i.key
const itemLabel = (i: PickerItem) => i.label
const itemPath = (i: PickerItem) => i.path

interface Data { tasks: Task[]; labels: Label[]; projects: Project[]; sections: Section[]; memberships: Map<string, Membership>; runningTask?: string | null; dayParts?: boolean
  /** Energy levels on (task-energy D3): the Energy submenu. */
  energy?: boolean
  /** Today with a limit (today-closed-list D3): the one task's slot command. */
  slot?: SlotState
  /** Today's focus (phone-touch-focus D3): the one task's focus command. */
  focus?: FocusMenuState }

/**
 * Start or Stop tracking for one task (time-tracking D5): Stop while its entry runs,
 * Start for an open task the user may edit; nothing for a completed or deleted one.
 */
export function trackingItem(task: Task | undefined, runningTask: string | null | undefined, memberships: Map<string, Membership>): ActionGroup['items'] {
  if (!task || task.deleted_at || roleOf(task.project_id, memberships) === 'viewer') return []
  const stop = runningTask === task.id
  if (!stop && task.completed_at) return []
  return [{ kind: 'action', id: 'track', label: stop ? t('Stop tracking') : t('Start tracking'), icon: 'timer', shortcut: keysOf('list.track'),
    onSelect: () => act(toggleTracking(task.id)) }]
}

/**
 * Start focus for one open task the user may edit (focus-everywhere D2): opens
 * the Focus screen on it with a session running (Countdown asks for the length).
 */
export function focusItem(task: Task | undefined, memberships: Map<string, Membership>): ActionGroup['items'] {
  if (!task || task.deleted_at || task.completed_at || roleOf(task.project_id, memberships) === 'viewer') return []
  return [{ kind: 'action', id: 'focus', label: t('Start focus'), icon: 'clock', onSelect: () => act(startFocusOn(task.id)) }]
}

/**
 * Take a slot / Give back the slot (today-closed-list D3): on Today with a limit, for a
 * waiting task or one let in. The list is personal, so a viewer of a shared project may too.
 */
export function slotItem(task: Task | undefined, slot: SlotState | undefined): ActionItem[] {
  const run = task && slot ? slotAction(task.id, slot) : null
  if (!task || !slot || !run) return []
  return [{ kind: 'action', id: 'slot', label: slotLabel(slot), icon: slot === 'waiting' ? 'arrow-up' : 'undo-2', onSelect: run }]
}

/**
 * Add to focus / Put in a slot / Remove from focus (phone-touch-focus D3): today's focus
 * is personal, so a viewer of a shared project may too.
 */
export function todayFocusItem(task: Task | undefined, state: FocusMenuState | undefined): ActionItem[] {
  if (!task || !state) return []
  return [{ kind: 'action', id: 'today-focus', label: focusMenuLabel(state), icon: state === 'remove' ? 'x' : 'sun', onSelect: () => runFocusPress(task.id) }]
}

/**
 * "Day part…" (day-parts D4): while the account uses day parts, a submenu of the
 * four parts, the tasks' common part checked; nothing for a viewer's task.
 */
export function dayPartItem(ids: readonly string[], d: Pick<Data, 'tasks' | 'memberships' | 'dayParts'>): ActionItem[] {
  if (!d.dayParts || d.tasks.length === 0 || d.tasks.some((task) => roleOf(task.project_id, d.memberships) === 'viewer')) return []
  const parts = new Set(d.tasks.map((task) => dayPartOf(task)))
  const common = parts.size === 1 ? [...parts][0] : undefined
  const set = (part: DayPart | null) => async () => {
    const r = await dayPartUndoable(ids, part)
    if (r.entry) showUndoToast(r.label, r.entry.id)
  }
  return [{ kind: 'submenu', id: 'day-part', label: t('Day part'), icon: 'sun', items: ([null, ...DAY_PARTS] as (DayPart | null)[]).map((part) => ({
    kind: 'check' as const, id: `part-${part ?? 'none'}`, label: dayPartName(part), checked: common === part, onSelect: set(part),
  })) }]
}

/**
 * "Energy" (task-energy D6): while the account has Energy levels on, a submenu of
 * Deep, Medium, Low and None, the tasks' common energy checked; nothing for a viewer's task.
 */
export function energyItem(ids: readonly string[], d: Pick<Data, 'tasks' | 'memberships' | 'energy'>): ActionItem[] {
  if (!d.energy || d.tasks.length === 0 || d.tasks.some((task) => roleOf(task.project_id, d.memberships) === 'viewer')) return []
  const levels = new Set(d.tasks.map((task) => energyOf(task)))
  const common = levels.size === 1 ? [...levels][0] : undefined
  const set = (e: Energy | null) => async () => {
    const r = await energyUndoable(ids, e)
    if (r.entry) showUndoToast(r.label, r.entry.id)
  }
  return [{ kind: 'submenu', id: 'energy', label: t('Energy'), icon: 'zap', items: ([...ENERGY_LEVELS, null] as (Energy | null)[]).map((e) => ({
    kind: 'check' as const, id: `energy-${e ?? 'none'}`, label: e ? energyName(e) : t('No energy'), checked: common === e, onSelect: set(e),
  })) }]
}

/** The menu's groups for the given tasks (exported for the palette and tests). */
export function bulkMenuGroups(ids: readonly string[], d: Data, a: { review: (title: string, plan: Promise<ReviewPlan>) => void; prefs: ResolvedDateTime; schedule?: () => void }, besideRow = false): ActionGroup[] {
  const n = d.tasks.length
  const reopen = allCompleted(d.tasks)
  const live = d.labels.filter((l) => !l.deleted_at)
  const common = live.filter((l) => d.tasks.length > 0 && d.tasks.every((task) => task.labels.includes(l.id))).map((l) => l.id)
  const projectId = d.tasks[0]?.project_id ?? null
  const oneProject = n > 0 && d.tasks.every((tk) => (tk.project_id ?? null) === projectId)
  const shared = commonSharedProject(d.tasks, d.memberships)
  const members = shared ? d.memberships.get(shared)?.members ?? [] : []
  const run = (action: BulkAction) => () => runBulk(ids, action)
  /** The priority every ticked task shares, shown checked in the digit row. */
  const sharedPriority = d.tasks.length > 0 && d.tasks.every((tk) => tk.priority === d.tasks[0]?.priority) ? d.tasks[0]?.priority ?? null : null
  const commonDue = d.tasks.length > 0 && d.tasks.every((tk) => tk.due?.date === d.tasks[0]?.due?.date) ? d.tasks[0]?.due?.date ?? null : null
  const complete: ActionItem = { kind: 'action', id: 'complete', label: reopen ? t('Reopen') : t('Complete'), icon: 'check', shortcut: keysOf('list.complete'), onSelect: run({ kind: 'complete' }) }
  // On touch, one task's menu starts with the two swipe actions (phone-v2 D3, WCAG 2.2 2.5.7).
  const swipeTwins = n === 1 && a.schedule
  return [
    ...(swipeTwins ? [{ items: [complete, { kind: 'action', id: 'schedule', label: t('Schedule…'), icon: 'calendar-days', onSelect: a.schedule } as ActionItem] }] : []),
    ...(n === 1 ? [{ items: [...slotItem(d.tasks[0], d.slot), ...todayFocusItem(d.tasks[0], d.focus), ...trackingItem(d.tasks[0], d.runningTask, d.memberships), ...focusItem(d.tasks[0], d.memberships)] }] : []),
    ...(swipeTwins ? [] : [{ items: [complete] }]),
    // One strip of digits instead of four rows (task-menu-refinement: cuts three rows);
    // the accessible names stay "Priority N", the shared one is checked. Beside a row
    // its own Priority and Schedule buttons already offer both.
    ...(besideRow ? [] : [
      { label: t('Priority'), layout: 'row', items: ([1, 2, 3, 4] as Priority[]).map((p) => ({
        kind: 'check', id: `p${p}`, label: t('Priority {p}', { p }),
        compact: { glyph: String(p), tint: `p${p}` }, checked: sharedPriority === p,
        onSelect: run({ kind: 'priority', priority: p }),
      })) },
      { label: t('Date'), items: [
        { kind: 'action', id: 'today', label: t('Today'), icon: 'calendar-check', onSelect: run({ kind: 'due', pick: 'today' }) },
        { kind: 'action', id: 'tomorrow', label: t('Tomorrow'), icon: 'calendar-days', onSelect: run({ kind: 'due', pick: 'tomorrow' }) },
        { kind: 'action', id: 'next-week', label: t('Next week'), icon: 'calendar-range', onSelect: run({ kind: 'due', pick: 'next-week' }) },
        { kind: 'action', id: 'no-date', label: t('No date'), icon: 'x', onSelect: run({ kind: 'due', pick: null }) },
        // The panel extends this overlay beside the items (task-menu-refinement D2):
        // Escape steps back to the items; a picked day applies and closes the menu.
        { kind: 'panel', id: 'pick-date', label: t('Pick a date…'), icon: 'calendar-days', panel: {
          title: t('Pick a date'),
          render: (close) => <DatePanel ids={ids} value={commonDue} prefs={a.prefs} onDone={close} />,
        } },
      ] },
    ] as ActionGroup[]),
    { items: [
      ...energyItem(ids, d),
      ...dayPartItem(ids, d),
      { kind: 'picker', id: 'labels', label: t('Labels'), icon: 'tag', shortcut: keysOf('list.labels'), disabled: live.length === 0, reason: t('There are no labels yet'), picker: {
        title: t('Labels'), placeholder: t('Add to all, or remove from all…'), source: live, multiple: true, selected: common, rankContext: 'label',
        getKey: (l: Label) => l.id, getLabel: (l: Label) => l.name,
        // Add or remove is decided from the ticked tasks as stored at the click, not from the
        // menu's read, which can still be loading on a quick pick; the open picker follows the
        // read, so its marks appear when it lands and change after each pick.
        onPick: (l: Label) => { void labelToggle(ids, l.id) },
      } },
    ] },

    { items: [
      { kind: 'picker', id: 'move', label: t('Move to project'), icon: 'folder-input', picker: {
        title: t('Move to project'), placeholder: t('Find a project…'), source: projectSource(d.projects, { memberships: d.memberships }),
        getKey: itemKey, getLabel: itemLabel, getPath: itemPath, getKeywords: itemKeywords, rankContext: 'project',
        onPick: (i: PickerItem) => { a.review(i.value ? t('Move {tasks} to {target}', { tasks: tasksWord(n), target: i.label }) : t('Move {tasks} to the Inbox', { tasks: tasksWord(n) }), reviewMove(ids, { projectId: i.value })) },
      } },
      { kind: 'picker', id: 'section', label: t('Move to section'), icon: 'kanban', disabled: !oneProject || !projectId, reason: t('The tasks must be in one project'), picker: {
        title: t('Move to section'), placeholder: t('Find a section…'), source: sectionSource(d.sections, projectId),
        getKey: itemKey, getLabel: itemLabel,
        onPick: (i: PickerItem) => { a.review(i.value ? t('Move {tasks} to {target}', { tasks: tasksWord(n), target: i.label }) : t('Move {tasks} to “No section”', { tasks: tasksWord(n) }), reviewMove(ids, { projectId, sectionId: i.value })) },
      } },
      { kind: 'picker', id: 'assign', label: t('Assign'), icon: 'user-check', shortcut: keysOf('list.assign'), disabled: !shared, reason: t('Tasks must be in one shared project'), picker: {
        title: t('Assign'), placeholder: t('Find a member…'), source: [{ userId: '', name: t('Unassigned'), email: '' }, ...members] as { userId: string; name: string; email: string }[],
        getKey: (m: { userId: string }) => m.userId || 'none', getLabel: (m: { name: string; email: string }) => m.name || m.email, rankContext: `assignee:${shared ?? ''}`,
        onPick: (m: { userId: string; name: string; email: string }) => { a.review(m.userId ? t('Assign {tasks} to {name}', { tasks: tasksWord(n), name: m.name || m.email }) : t('Unassign {tasks}', { tasks: tasksWord(n) }), reviewAssign(ids, m.userId || null)) },
      } },
    ] },
    { items: [{ kind: 'action', id: 'trash', label: t('Move to Trash'), icon: 'trash-2', shortcut: keysOf('list.delete'), tone: 'danger', onSelect: run({ kind: 'trash' }) }] },
  ]
}

/** Removes the label when every ticked task has it, otherwise adds it to the ones lacking it. */
async function labelToggle(ids: readonly string[], labelId: string) {
  const { tasks } = await readTicked(ids)
  const all = tasks.length > 0 && tasks.every((task) => task.labels.includes(labelId))
  await runBulk(ids, { kind: 'label', labelId, add: !all })
}

/**
 * The date panel (task-menu-refinement "Date picker alongside the menu"): a phrase
 * typed on top (commit on Enter or blur, like the date field) over the month grid,
 * rendered beside the menu's items — not in a dialog on top of them. Picking closes
 * the menu and reschedules every id, with the Undo toast.
 */
function DatePanel({ ids, value, prefs, onDone }: { ids: readonly string[]; value: string | null; prefs: ResolvedDateTime; onDone: () => void }) {
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | null>(null)
  const apply = (iso: string | null) => { onDone(); if (iso) void runBulk(ids, { kind: 'due', pick: iso }) }
  const commit = () => {
    const text = draft.trim()
    if (!text) { setError(null); return }
    const iso = parseDateInput(text, { today: logicalToday(), prefs, languages: inputLanguages() })
    if (!iso) {
      setError(t('Not a valid date. Try {example}, {iso} or “tomorrow”.', { example: formatDate(logicalToday(), 'numeric', prefs), iso: logicalToday() }))
      return
    }
    setDraft('')
    setError(null)
    apply(iso)
  }
  return (
    <div className="space-y-2" data-date-panel>
      <Input value={draft} autoFocus autoComplete="off" spellCheck={false} aria-label={t('Date')}
        placeholder={formatDate(logicalToday(), 'numeric', prefs)}
        onChange={(e) => setDraft(e.target.value)} onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === 'Enter') { e.preventDefault(); commit() }
          // First Escape clears the draft (the date field's rule); the next reaches the panel.
          else if (e.key === 'Escape' && draft !== '') { e.stopPropagation(); setDraft(''); setError(null) }
        }} />
      {error && <p role="alert" className="text-meta text-danger">{error}</p>}
      <CalendarGrid value={value} prefs={prefs} onPick={apply} compact />
    </div>
  )
}

/** Live data the menu needs, read while it is mounted. */
function useBulkData(ids: readonly string[]): Data {
  const key = ids.join(',')
  // Tagged with the selection it was read for: until the read for this selection lands,
  // the menu has no tasks (and marks no common label) rather than an earlier selection's.
  const read = useTickedTasks(ids, key)
  const tasks = useMemo(() => (read?.key === key ? read.tasks : []), [read, key])
  const labels = useAllLabels()
  const projects = useProjects()
  const sections = useLiveSections()
  const memberships = useMembershipMap()
  const runningTask = useRunningEntry()?.task_id ?? null
  const prefs = useUserPrefsRow()
  const dayParts = dayPartsOn(prefs)
  const energy = energyLevelsOn(prefs)
  const slot = useSlotState(ids.length === 1 ? ids[0] : '')
  const review = useReviewRow()
  const today = useToday()
  const focus = tasks.length === 1 ? focusMenuState(tasks[0], review, prefs, today) : null
  return { tasks, labels, projects, sections, memberships, runningTask, dayParts, energy, slot, focus }
}

/**
 * The Actions menu for `ids`. `startPicker` opens it straight into a picker
 * ("labels" for `L`, "assign" for Shift+A on a selection).
 *
 * `onAllClosed` (task-menu-refinement D1): a host that renders this menu
 * conditionally — the right-click and `.`-cursor menus — must keep it mounted
 * until both the menu and every dialog it opened are gone, or the review
 * dialog dies with the component whose state holds it. The host clears its
 * entry when this fires.
 */
export function BulkActionsMenu({ ids, open, onOpenChange, trigger, triggerClassName, triggerProps, startPicker = null, onApplied, onAllClosed, anchor }: {
  ids: readonly string[]
  open?: boolean
  onOpenChange?: (open: boolean) => void
  trigger?: React.ReactNode
  triggerClassName?: string
  /** Extra attributes on the trigger button (the pointer position of a right-click menu). */
  triggerProps?: React.ComponentProps<typeof ActionMenu>['triggerProps']
  startPicker?: string | null
  onApplied?: () => void
  /** The menu and everything it opened are closed: the host may unmount it now. */
  onAllClosed?: () => void
  /** Opened from a row's More button: placed there, without the groups the row's own buttons offer. */
  anchor?: RefObject<HTMLElement | null>
}) {
  const data = useBulkData(ids)
  const [review, setReview] = useState<{ title: string; plan: ReviewPlan } | null>(null)
  /** A review's plan resolves asynchronously after the menu already closed; hold until it lands. */
  const [reviewing, setReviewing] = useState(false)
  const { resolved } = useDateTimeFormat()
  const besideRow = !!anchor?.current
  const touch = !useFinePointer()
  const groups = useMemo(() => bulkMenuGroups(ids, data, {
    review: (title, plan) => {
      setReviewing(true)
      void plan.then((p) => { setReview({ title, plan: p }); setReviewing(false) }, () => setReviewing(false))
    },
    prefs: resolved,
    schedule: touch && ids.length === 1 ? () => openRowSchedule(ids[0]) : undefined,
  }, besideRow), [ids, data, besideRow, touch, resolved])
  // D1: only report "all closed" when nothing this menu opened is still on screen.
  // The date panel lives inside the menu itself, so an open menu already covers it.
  const allClosed = open === false && !review && !reviewing
  useEffect(() => { if (allClosed) onAllClosed?.() }, [allClosed, onAllClosed])
  const n = data.tasks.length || ids.length
  return (
    <>
      <ActionMenu label={t('Actions for {tasks}', { tasks: tasksWord(n) })} title={tasksWord(n)} groups={groups} open={open} onOpenChange={onOpenChange}
        trigger={trigger} triggerClassName={triggerClassName} triggerProps={triggerProps} startPicker={startPicker} width="sm:w-72" anchor={anchor} />
      {review && <BulkReview title={review.title} plan={review.plan} onClose={() => setReview(null)} onApplied={onApplied} />}
    </>
  )
}

/** "3 selected · Actions · Clear" above a list while tasks are ticked, kept in sight while the list scrolls. */
export function SelectionBar({ ids, onClear, menuOpen, onMenuOpenChange, startPicker, onDone }: {
  ids: readonly string[]
  onClear: () => void
  menuOpen: boolean
  onMenuOpenChange: (open: boolean) => void
  startPicker?: string | null
  /** Select mode on touch (phone-v2 D8): Done hides the checkboxes and keeps the ticks; the bar shows with nothing ticked. */
  onDone?: () => void
}) {
  const bar = useRef<HTMLDivElement>(null)
  const shown = ids.length > 0 || !!onDone
  // Pinned below the page header, so it stays reachable anywhere in a long list (issue 518).
  const top = useBelowStickyHeader(bar, shown)
  if (!shown) return null
  return (
    <div ref={bar} style={{ top }} className="sticky z-10 mb-2 flex items-center gap-2 rounded-control bg-selected px-3 py-1.5 text-body shadow-2" data-selection-bar>
      <span role="status" className="min-w-0 flex-1 font-medium text-strong">{t('{count} selected', { count: ids.length })}</span>
      {ids.length > 0 && <BulkActionsMenu ids={ids} open={menuOpen} onOpenChange={onMenuOpenChange} startPicker={startPicker}
        trigger={<>{t('Actions')} <Kbd k={keysOf('list.actions')} only className="ml-1" /></>} triggerClassName="ui-btn ui-btn--ghost ui-btn--sm px-2 py-1 text-body" />}
      {ids.length > 0 && (
        <Tooltip label={t('Clear the selection')} shortcut="Esc">
          <button type="button" className="ui-btn ui-btn--ghost ui-btn--sm px-2 py-1 text-body" onClick={onClear}><Icon name="x" />{t('Clear')}</button>
        </Tooltip>
      )}
      {onDone && <button type="button" className="ui-btn ui-btn--ghost ui-btn--sm touch-target px-2 py-1 text-body font-medium" onClick={onDone} data-select-done>{t('Done')}</button>}
    </div>
  )
}
