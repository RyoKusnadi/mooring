// Hands the links termlinks.ts finds to xterm: a click opens a web address in a new browser tab,
// or a project file in the editor at its line (through terms.links, set by the shell).
import type { IDisposable, ILink, Terminal } from '@xterm/xterm'
import { findLinks } from './termlinks'
import { terms } from './termstore.svelte'

export function addLinks(term: Terminal, id: string): IDisposable {
  return term.registerLinkProvider({
    provideLinks(y, done) {
      const text = term.buffer.active.getLine(y - 1)?.translateToString(true) ?? ''
      const links: ILink[] = []
      for (const l of findLinks(text)) {
        const range = { start: { x: l.start + 1, y }, end: { x: l.end, y } }
        if (l.url) {
          const url = l.url
          links.push({ range, text: l.text, activate: () => window.open(url, '_blank', 'noopener,noreferrer') })
          continue
        }
        const to = terms.links
        const path = to && l.path ? to.path(id, l.path) : null
        if (to && path) links.push({ range, text: l.text, activate: () => to.open(path, l.line) })
      }
      done(links.length ? links : undefined)
    },
  })
}
