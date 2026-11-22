// How the language features look: tooltips, the completion list, lint marks and the gutter. The
// libraries draw them with fixed colours; this puts the app's tokens on top (DESIGN.md). Meaning
// never rests on colour alone: errors, warnings and notes differ in shape and in the line style.
import { EditorView } from '@codemirror/view'

const underline = (style: string, colour: string) => ({ backgroundImage: 'none !important', textDecoration: `underline ${style} ${colour}`, textDecorationThickness: '1.5px', textUnderlineOffset: '2px', paddingBottom: '0' })
const marker = (colour: string, shape?: string) => ({ content: 'none', display: 'block', width: '0.8em', height: '0.8em', margin: '0.25em 0', background: colour, ...(shape ? { clipPath: shape } : {}) })

export const lspTheme = EditorView.theme({
  '.cm-tooltip': { backgroundColor: 'var(--raised)', color: 'var(--fg)', border: '1px solid var(--border-strong)', borderRadius: 'var(--r-m)', fontFamily: 'var(--font-ui)', fontSize: 'var(--fs-m)', boxShadow: '0 4px 16px rgb(0 0 0 / 0.2)' },
  '.cm-tooltip.cm-tooltip-hover, .cm-lsp-signature-tooltip': { maxWidth: 'min(48ch, 92vw)', overflowX: 'hidden', overflowWrap: 'anywhere' },
  '.cm-lsp-documentation': { padding: 'var(--s-1) var(--s-2)' },
  '.cm-lsp-documentation pre, .cm-lsp-documentation code, .cm-lsp-signature': { fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-s)', whiteSpace: 'pre-wrap' },
  '.cm-lsp-active-parameter': { color: 'var(--accent)', textDecoration: 'underline' },
  '.cm-tooltip.cm-tooltip-autocomplete > ul': { fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-s)', maxHeight: 'min(16em, 40dvh)' },
  '.cm-tooltip.cm-tooltip-autocomplete > ul > li': { padding: '2px var(--s-2)' },
  '.cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected]': { backgroundColor: 'var(--selected)', color: 'var(--fg)', outline: '1px solid var(--border-strong)', outlineOffset: '-1px' },
  '.cm-completionDetail': { color: 'var(--muted)', fontStyle: 'normal', marginLeft: 'var(--s-2)' },
  '.cm-completionMatchedText': { textDecoration: 'underline', fontWeight: '600' },
  '.cm-completionIcon': { display: 'none' },
  '.cm-diagnostic': { padding: 'var(--s-1) var(--s-2)', fontFamily: 'var(--font-ui)', fontSize: 'var(--fs-m)' },
  '.cm-diagnostic-error': { borderLeft: '4px solid var(--danger)' },
  '.cm-diagnostic-warning': { borderLeft: '4px solid var(--attention)' },
  '.cm-diagnostic-info, .cm-diagnostic-hint': { borderLeft: '4px solid var(--border-strong)' },
  '&.cm-editor .cm-lintRange-error': underline('wavy', 'var(--danger)'),
  '&.cm-editor .cm-lintRange-warning': underline('dashed', 'var(--attention)'),
  '&.cm-editor .cm-lintRange-info, &.cm-editor .cm-lintRange-hint': underline('dotted', 'var(--muted)'),
  '.cm-lintRange-active': { backgroundColor: 'var(--hover)' },
  '.cm-lintPoint:after': { borderBottomColor: 'var(--danger)' },
  '.cm-gutter-lint': { width: '1.4em' },
  '.cm-lint-marker-error': marker('var(--danger)', 'circle(50%)'),
  '.cm-lint-marker-warning': marker('var(--attention)', 'polygon(50% 0, 100% 100%, 0 100%)'),
  '.cm-lint-marker-info': marker('var(--muted)'),
  '.cm-panel.cm-panel-lint ul [aria-selected]': { backgroundColor: 'var(--selected)', color: 'var(--fg)' },
  '.cm-panel.cm-panel-lint ul:focus [aria-selected]': { backgroundColor: 'var(--selected)', color: 'var(--fg)', outline: '2px solid var(--accent)', outlineOffset: '-2px' },
  '.cm-panel.cm-panel-lint [name=close]': { color: 'var(--muted)', cursor: 'pointer' },
  '.cm-lsp-message': { fontFamily: 'var(--font-ui)', padding: 'var(--s-1) var(--s-2)' },
})
