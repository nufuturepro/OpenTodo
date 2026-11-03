// The grip's reminder (OpenSpec change version-hint-motion): a card above the first
// list that has tasks, on touch devices, until this device dismisses it — once per
// device through deviceStorage, like RowSwipe's swipe cue, and only where the gesture
// happens (a fine pointer never needs it; TaskListView gates it).
import { useState, type ReactNode } from 'react'
import { Notice } from '../../ui'
import { t } from '../../lib/i18n'
import { readString, writeString } from '../../lib/deviceStorage'

const HINT_KEY = 'opentodo.grip-hint'

/** Whether this device already dismissed the hint (storage refused: it shows again next launch). */
export function gripHintDismissed(): boolean {
  return readString(HINT_KEY) === '1'
}

/** Press and hold a task's grip to select it. */
export function GripHint(): ReactNode {
  const [dismissed, setDismissed] = useState(gripHintDismissed)
  if (dismissed) return null
  return (
    <div className="hint-in mb-3" data-grip-hint>
      <Notice action={
        <button type="button" className="ui-btn ui-btn--secondary ui-btn--sm" data-grip-hint-dismiss
          onClick={() => { setDismissed(true); writeString(HINT_KEY, '1') }}>
          {t('Got it')}
        </button>
      }>
        {t('Press and hold a task’s grip to select it.')}
      </Notice>
    </div>
  )
}
