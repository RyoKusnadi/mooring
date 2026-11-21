// The editor itself: CodeMirror 6 set up with Mooring's tokens. Only the file view imports this,
// so it all lives in the lazy editor chunk.
import { Annotation, Compartment, EditorState, type Extension, type Text } from '@codemirror/state'
import { EditorView, drawSelection, highlightActiveLine, highlightActiveLineGutter, keymap, lineNumbers, type ViewUpdate } from '@codemirror/view'
import { defaultKeymap, history, historyKeymap, indentLess, indentMore } from '@codemirror/commands'
import { HighlightStyle, bracketMatching, indentOnInput, syntaxHighlighting } from '@codemirror/language'
import { highlightSelectionMatches, searchKeymap } from '@codemirror/search'
import { tags as t } from '@lezer/highlight'

/** Marks changes that come from the disk, so they do not count as the user's edits. */
export const fromDisk = Annotation.define<boolean>()

export const lang = new Compartment()
export const editable = new Compartment()
export const separator = new Compartment()
/** Language server features; empty until a server connection is up. */
export const lsp = new Compartment()

/** Tab indents, so it would trap the keyboard. Esc followed by Tab hands Tab back to the page (announced in the file view). */
const escaped = new WeakSet<EditorView>()
const leave = (run: (v: EditorView) => boolean) => (v: EditorView) => (escaped.delete(v) ? false : run(v))
const tabKeys = [
  { key: 'Escape', run: (v: EditorView) => (escaped.add(v), false) },
  { key: 'Tab', run: leave(indentMore) },
  { key: 'Shift-Tab', run: leave(indentLess) },
]

const theme = EditorView.theme({
  '&': { height: '100%', backgroundColor: 'var(--bg)', color: 'var(--fg)', fontSize: 'var(--fs-m)' },
  '.cm-scroller': { fontFamily: 'var(--font-mono)', lineHeight: '1.5', overflow: 'auto' },
  '.cm-content': { caretColor: 'var(--accent)', padding: 'var(--s-2) 0' },
  '.cm-cursor': { borderLeftColor: 'var(--accent)' },
  '&.cm-focused': { outline: '2px solid var(--accent)', outlineOffset: '-2px' },
  '.cm-gutters': { backgroundColor: 'var(--bg)', color: 'var(--muted)', border: 'none' },
  '.cm-activeLine': { backgroundColor: 'var(--hover)' },
  '.cm-activeLineGutter': { backgroundColor: 'var(--hover)', color: 'var(--fg)' },
  '.cm-selectionBackground, &.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, ::selection': { backgroundColor: 'var(--selected)' },
  '.cm-selectionMatch': { backgroundColor: 'var(--hover)', outline: '1px solid var(--border-strong)' },
  '.cm-matchingBracket': { outline: '1px solid var(--border-strong)', backgroundColor: 'transparent' },
  '.cm-panels': { backgroundColor: 'var(--surface)', color: 'var(--fg)', borderTop: '1px solid var(--border)', fontFamily: 'var(--font-ui)' },
  '.cm-search': { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 'var(--s-1)', padding: 'var(--s-1) var(--s-2)' },
  '.cm-search br': { display: 'none' },
  '.cm-search label': { fontSize: 'var(--fs-s)' },
  '.cm-textfield': { minHeight: 'var(--control-h)', padding: '0 var(--s-2)', border: '1px solid var(--border-strong)', borderRadius: 'var(--r-s)', backgroundColor: 'var(--raised)', color: 'var(--fg)', font: 'inherit' },
  '.cm-button': { minHeight: 'var(--control-h)', padding: '0 var(--s-2)', border: '1px solid var(--border-strong)', borderRadius: 'var(--r-s)', backgroundColor: 'transparent', backgroundImage: 'none', color: 'var(--fg)', font: 'inherit' },
  '.cm-panel.cm-search [name=close]': { color: 'var(--muted)', backgroundColor: 'transparent', border: '0', cursor: 'pointer', font: 'inherit', fontSize: 'var(--fs-m)' },
  '.cm-searchMatch': { backgroundColor: 'var(--attention-bg)', outline: '1px solid var(--attention)' },
  '.cm-searchMatch.cm-searchMatch-selected': { backgroundColor: 'var(--selected)', outline: '2px solid var(--accent)' },
})

// Colours come from the terminal palette tokens, which pass 4.5:1 on --bg in both themes.
const colours = HighlightStyle.define([
  { tag: [t.keyword, t.operatorKeyword, t.modifier, t.controlKeyword], color: 'var(--ansi-5)' },
  { tag: [t.string, t.special(t.string), t.regexp], color: 'var(--ansi-2)' },
  { tag: [t.number, t.bool, t.null, t.atom], color: 'var(--ansi-4)' },
  { tag: [t.comment, t.lineComment, t.blockComment, t.docComment], color: 'var(--muted)', fontStyle: 'italic' },
  { tag: [t.typeName, t.className, t.namespace, t.tagName], color: 'var(--ansi-6)' },
  { tag: [t.function(t.variableName), t.function(t.propertyName), t.definition(t.variableName)], color: 'var(--fg)', fontWeight: '600' },
  { tag: [t.propertyName, t.attributeName, t.labelName], color: 'var(--ansi-3)' },
  { tag: [t.heading, t.strong], fontWeight: '600' },
  { tag: t.emphasis, fontStyle: 'italic' },
  { tag: [t.link, t.url], color: 'var(--accent)', textDecoration: 'underline' },
  { tag: t.invalid, color: 'var(--danger)' },
])

export interface EditorHooks {
  onChange: (doc: Text, userEdit: boolean) => void
  onSave: () => void
  /** The selection moved or its document changed. */
  onSelect?: (state: EditorState) => void
}

export function createEditor(parent: HTMLElement, doc: string, h: EditorHooks): EditorView {
  const update = (u: ViewUpdate) => {
    if (u.selectionSet || u.docChanged) h.onSelect?.(u.state)
    if (u.docChanged) h.onChange(u.state.doc, !u.transactions.every((tr) => tr.annotation(fromDisk)))
  }
  const extensions: Extension[] = [
    lineNumbers(),
    highlightActiveLineGutter(),
    highlightActiveLine(),
    drawSelection(),
    history(),
    indentOnInput(),
    bracketMatching(),
    highlightSelectionMatches(),
    syntaxHighlighting(colours),
    theme,
    lang.of([]),
    editable.of([]),
    separator.of([]),
    lsp.of([]),
    // Cmd/Ctrl+S saves; the browser's own "save page" never runs.
    keymap.of([{ key: 'Mod-s', preventDefault: true, run: () => (h.onSave(), true) }, ...searchKeymap, ...historyKeymap, ...defaultKeymap, ...tabKeys]),
    EditorView.domEventHandlers({ keydown: (e, v) => void (e.key !== 'Escape' && e.key !== 'Tab' && e.key !== 'Shift' && escaped.delete(v)) }),
    // Long lines wrap on a phone, where there is no room to scroll sideways.
    ...(typeof matchMedia === 'function' && matchMedia('(max-width: 900px)').matches ? [EditorView.lineWrapping] : []),
    EditorView.contentAttributes.of({ 'aria-label': 'File contents', spellcheck: 'false' }),
    EditorView.updateListener.of(update),
  ]
  return new EditorView({ parent, state: EditorState.create({ doc, extensions }) })
}

export const readOnly = (on: boolean): Extension => [EditorState.readOnly.of(on), EditorView.editable.of(!on)]
export const lineEnding = (eol: string): Extension => (eol === '\n' ? [] : EditorState.lineSeparator.of(eol))
