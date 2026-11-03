// Settings → About's version button (OpenSpec change version-hint-motion): this
// repository's changelog, compiled by scripts/changelog-build.mjs into a lazy module
// (generated/changelog.json), newest release first. The prose keeps its **bold**, `code`
// and [links](url) through the small renderer below, and every section heading goes
// through t() so the changelog reads in the app's language. The dialog's entrance is the
// shared dialog motion — a pop-up on desktop, a slide-up sheet on phones — and is
// instant under data-motion="none" (appearance-preferences "Motion preference") or the
// system's reduced motion, like every other overlay.
import { useEffect, useState, type ReactNode } from 'react'
import { Dialog } from '../ui'
import { t } from '../lib/i18n'

type Release = { version: string; date?: string; sections: { kind: string; items: string[] }[] }
type Changelog = { releases: Release[] }

/** The section headings of CHANGELOG.md (Added, Changed, …) in the app's language. */
function kindLabel(kind: string): string {
  switch (kind) {
    case 'Added': return t('Added')
    case 'Breaking': return t('Breaking')
    case 'Changed': return t('Changed')
    case 'Fixed': return t('Fixed')
    case 'Highlights': return t('Highlights')
    case 'Known limitations and verification follow-ups': return t('Known limitations and verification follow-ups')
    case 'Migrations': return t('Migrations')
    case 'Security': return t('Security')
    case 'Upgrade notes': return t('Upgrade notes')
    default: return kind
  }
}

/** The changelog's inline subset: **bold**, `code` and [label](url); everything else is plain text. */
function Inline({ text }: { text: string }): ReactNode {
  const parts: ReactNode[] = []
  const pattern = /\*\*([^*]+)\*\*|`([^`]+)`|\[([^\]]+)\]\(([^)\s]+)\)/g
  let taken = 0
  for (const m of text.matchAll(pattern)) {
    const at = m.index ?? 0
    if (at > taken) parts.push(text.slice(taken, at))
    if (m[1] !== undefined) parts.push(<strong key={at}>{m[1]}</strong>)
    else if (m[2] !== undefined) parts.push(<code key={at}>{m[2]}</code>)
    else parts.push(<a key={at} href={m[4]} target="_blank" rel="noopener noreferrer">{m[3]}</a>)
    taken = at + m[0].length
  }
  if (taken < text.length) parts.push(text.slice(taken))
  return <>{parts}</>
}

/** Opens from the About section's version row; the compiled changelog arrives as its own lazy chunk. */
export function ChangelogDialog({ onClose }: { onClose: () => void }): ReactNode {
  const [data, setData] = useState<Changelog | null>(null)
  useEffect(() => {
    let alive = true
    void import('../generated/changelog.json')
      .then((mod) => { if (alive) setData((mod as { default: unknown }).default as Changelog) })
      .catch(() => { if (alive) setData({ releases: [] }) })
    return () => { alive = false }
  }, [])
  return (
    <Dialog title={t('Changelog')} onClose={onClose} size="md" testId="changelog">
      {data === null ? (
        <p className="py-6 text-center text-meta text-muted">{t('Loading the changelog…')}</p>
      ) : data.releases.length === 0 ? (
        <p className="py-6 text-center text-meta text-muted">{t('The changelog could not be loaded.')}</p>
      ) : (
        <div className="flex flex-col gap-5 pb-2" data-changelog>
          {data.releases.map((release) => (
            <section key={release.version} aria-label={release.version}>
              <h3 className="text-body font-semibold" data-changelog-version={release.version}>
                {release.version}
                {release.date ? <span className="text-meta font-normal text-muted"> · {release.date}</span> : null}
              </h3>
              {release.sections.map((section) => (
                <div key={section.kind} className="mt-3">
                  <h4 className="text-meta text-muted">{kindLabel(section.kind)}</h4>
                  <ul className="mt-1 flex list-disc flex-col gap-1.5 pl-5 text-body text-muted">
                    {section.items.map((item, i) => <li key={`${section.kind}-${i}`}><Inline text={item} /></li>)}
                  </ul>
                </div>
              ))}
            </section>
          ))}
        </div>
      )}
    </Dialog>
  )
}
