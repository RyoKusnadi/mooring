// A CodeMirror gutter that names who last changed each line: the short id, the author and how long
// ago, once at the start of each run of lines. Loaded the first time blame is turned on.
import { StateEffect, StateField, type Extension } from '@codemirror/state'
import { EditorView, GutterMarker, gutter } from '@codemirror/view'
import { runAt, type Blame } from './blame'
import { relativeTime, shortId } from './historyapi'

/** The blame to draw, or null to hide the gutter. */
export const setBlame = StateEffect.define<Blame | null>()
const blameField = StateField.define<Blame | null>({
  create: () => null,
  update(v, tr) {
    for (const e of tr.effects) if (e.is(setBlame)) return e.value
    return v
  },
})
export const hasBlameGutter = (view: EditorView): boolean => view.state.field(blameField, false) !== undefined

const el = (tag: string, cls: string, text = ''): HTMLElement => {
  const n = document.createElement(tag)
  n.className = cls
  n.textContent = text
  return n
}

class Marker extends GutterMarker {
  constructor(
    readonly sha: string,
    readonly author: string,
    readonly time: number,
    readonly summary: string,
    readonly open: (sha: string) => void,
  ) {
    super()
  }
  eq(o: Marker): boolean {
    return o.sha === this.sha && o.author === this.author && o.time === this.time && o.summary === this.summary
  }
  toDOM(): HTMLElement {
    if (!this.sha) return el('span', 'cm-blame-row cm-blame-new', 'Not committed yet')
    const b = el('button', 'cm-blame-row') as HTMLButtonElement
    b.type = 'button'
    b.title = this.summary
    // The gutter is hidden from assistive technology and the bar has the keyboard path, so no tab stop here.
    b.tabIndex = -1
    b.append(el('span', 'cm-blame-id', shortId(this.sha)), el('span', 'cm-blame-who', this.author), el('span', 'cm-blame-age', relativeTime(this.time)))
    b.onclick = () => this.open(this.sha)
    return b
  }
}

class Continued extends GutterMarker {
  toDOM(): HTMLElement {
    return el('span', 'cm-blame-row cm-blame-more')
  }
}
const continued = new Continued()

/** What changed the line the cursor is on, in words; the gutter itself is hidden from screen readers. */
export function lineNote(b: Blame, line: number): string {
  const r = runAt(b, line)
  if (!r) return ''
  const c = b.commits[r.commit]
  if (!r.commit || !c) return `Line ${line}: not committed yet.`
  return `Line ${line}: ${c.summary}, by ${c.author}, ${relativeTime(c.time)} (${shortId(r.commit)}).`
}

/** `open` shows a commit; `note` hears about the cursor line. Add this once; `setBlame` then fills it and empties it. */
export function blameGutter(open: (sha: string) => void, note: (text: string, sha: string) => void): Extension {
  const cache = new Map<string, Marker>()
  return [
    blameField,
    EditorView.updateListener.of((u) => {
      const b = u.state.field(blameField, false) ?? null
      const moved = u.selectionSet || u.docChanged || (u.startState.field(blameField, false) ?? null) !== b
      if (!moved) return
      const n = u.state.doc.lineAt(u.state.selection.main.head).number
      note(b ? lineNote(b, n) : '', b ? (runAt(b, n)?.commit ?? '') : '')
    }),
    EditorView.editorAttributes.compute([blameField], (s) => ({ class: s.field(blameField) ? 'cm-blame-on' : '' })),
    gutter({
      class: 'cm-blame-gutter',
      lineMarkerChange: (u) => u.startState.field(blameField) !== u.state.field(blameField),
      lineMarker(view, line) {
        const b = view.state.field(blameField)
        if (!b) return null
        const n = view.state.doc.lineAt(line.from).number
        const r = runAt(b, n)
        if (!r) return null
        if (r.start !== n && line.from !== view.viewport.from) return continued
        // A run that began above the screen still names its commit on the first line shown.
        const key = `${r.commit}`
        const c = b.commits[r.commit]
        let m = cache.get(key)
        if (!m) cache.set(key, (m = new Marker(r.commit, c?.author ?? '', c?.time ?? 0, c?.summary ?? '', open)))
        return m
      },
    }),
    EditorView.baseTheme({
      '.cm-blame-gutter': { display: 'none' },
      '&.cm-blame-on .cm-blame-gutter': { display: 'block' },
      '.cm-blame-row': { display: 'flex', boxSizing: 'border-box', width: '30ch', gap: '1ch', padding: '0 var(--s-2)', overflow: 'hidden', whiteSpace: 'nowrap', textAlign: 'left', color: 'var(--muted)', font: 'inherit', fontSize: 'var(--fs-s)', border: 'none', borderLeft: '2px solid var(--border-strong)', background: 'none', cursor: 'pointer' },
      '.cm-blame-more, .cm-blame-new': { cursor: 'default' },
      '.cm-blame-more': { borderLeftColor: 'var(--border)' },
      'button.cm-blame-row:hover': { backgroundColor: 'var(--hover)', color: 'var(--fg)' },
      '.cm-blame-id': { flex: 'none' },
      '.cm-blame-who': { flex: '1 1 0', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' },
      '.cm-blame-age': { flex: 'none' },
      '@media (max-width: 700px), (pointer: coarse)': { '.cm-blame-row': { width: '19ch' }, '.cm-blame-who': { display: 'none' } },
    }),
  ]
}
