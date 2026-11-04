// How the changelog opens (this device): a dialog over the page, or embedded under
// the version row in Settings → About. A device preference like the Vim keybindings
// one — there is nothing to sync, the choice is about this screen — so it lives in
// localStorage and applies at once.
import { createStoredValue } from './deviceStorage'

export const NOTES_VIEW_KEY = 'opentodo.notes'

/** The two presentations: a pop-up dialog, or embedded in the About page. */
export const NOTES_VIEWS = ['popup', 'embed'] as const
export type NotesView = (typeof NOTES_VIEWS)[number]

const view = createStoredValue<NotesView>(NOTES_VIEW_KEY, {
  parse: (raw) => (raw === 'embed' ? 'embed' : 'popup'),
  serialize: (value) => (value === 'embed' ? 'embed' : null),
}, { crossTab: true, server: 'popup' })

/** 'popup' unless this device chose to embed the release notes. */
export const readNotesView = view.get

/** Chooses the presentation for this device; the About page follows at once. */
export const setNotesView = view.set

/** The presentation, re-rendering the caller when another tab changes it. */
export const useNotesView = view.use
