// Draws parsed markdown as DOM nodes. Every piece of text goes in through createTextNode or
// textContent, never innerHTML, so nothing the agent wrote can become markup or a script.
import type { Block, Inline } from './md'
import { classifyLink } from './mdlink'
import { highlight } from './mdhighlight'

export interface MdContext {
  /** Absolute path of the project folder, for links to files. */
  root: string
  /** Opens a project file; the page checks it with the server first. */
  openFile: (path: string, line?: number) => void
}

const el = <K extends keyof HTMLElementTagNameMap>(tag: K, cls = ''): HTMLElementTagNameMap[K] => {
  const n = document.createElement(tag)
  if (cls) n.className = cls
  return n
}

export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    const box = el('textarea')
    box.value = text
    box.style.position = 'fixed'
    box.style.opacity = '0'
    document.body.appendChild(box)
    box.select()
    const ok = document.execCommand('copy')
    box.remove()
    return ok
  }
}

export function renderInline(list: Inline[], ctx: MdContext, into: Node): void {
  for (const n of list) {
    if (n.t === 'text') into.appendChild(document.createTextNode(n.s))
    else if (n.t === 'br') into.appendChild(el('br'))
    else if (n.t === 'code') {
      const c = el('code', 'md-code')
      c.textContent = n.s
      into.appendChild(c)
    } else if (n.t === 'link') {
      const target = classifyLink(n.href, ctx.root)
      if (target.kind === 'web') {
        const a = el('a', 'md-link')
        a.href = target.href
        a.target = '_blank'
        a.rel = 'noopener noreferrer'
        renderInline(n.c, ctx, a)
        into.appendChild(a)
      } else if (target.kind === 'file') {
        const b = el('button', 'md-link md-file')
        b.type = 'button'
        b.title = `Open ${target.path}`
        b.addEventListener('click', () => ctx.openFile(target.path, target.line))
        renderInline(n.c, ctx, b)
        into.appendChild(b)
      } else renderInline(n.c, ctx, into)
    } else {
      const e = el(n.t === 'em' ? 'em' : n.t === 'strong' ? 'strong' : 'del')
      renderInline(n.c, ctx, e)
      into.appendChild(e)
    }
  }
}

function codeBlock(b: Extract<Block, { t: 'code' }>): HTMLElement {
  const box = el('div', 'md-pre')
  const bar = el('div', 'md-pre-bar')
  const name = el('span', 'md-lang')
  name.textContent = b.lang
  const copy = el('button', 'btn small ghost md-copy')
  copy.type = 'button'
  copy.textContent = 'Copy'
  copy.setAttribute('aria-label', b.lang ? `Copy ${b.lang} code` : 'Copy code')
  let timer: ReturnType<typeof setTimeout> | undefined
  copy.addEventListener('click', () => {
    void copyText(b.text).then((ok) => {
      copy.textContent = ok ? 'Copied' : 'Could not copy'
      clearTimeout(timer)
      timer = setTimeout(() => (copy.textContent = 'Copy'), 1500)
    })
  })
  bar.append(name, copy)
  const pre = el('pre')
  pre.tabIndex = 0
  const code = el('code')
  code.textContent = b.text
  pre.appendChild(code)
  box.append(bar, pre)
  // A fence that is still open is plain text; it is coloured once it closes.
  if (b.closed && b.lang) {
    void highlight(b.text, b.lang).then((runs) => {
      if (!runs || !code.isConnected) return
      const frag = document.createDocumentFragment()
      for (const r of runs) {
        if (!r.cls) frag.appendChild(document.createTextNode(r.text))
        else {
          const s = el('span', r.cls)
          s.textContent = r.text
          frag.appendChild(s)
        }
      }
      code.replaceChildren(frag)
    })
  }
  return box
}

function table(b: Extract<Block, { t: 'table' }>, ctx: MdContext): HTMLElement {
  const wrap = el('div', 'md-table')
  wrap.tabIndex = 0
  wrap.setAttribute('role', 'region')
  wrap.setAttribute('aria-label', 'Table')
  const t = el('table')
  const row = (cells: Inline[][], tag: 'th' | 'td', into: HTMLElement) => {
    const tr = el('tr')
    cells.forEach((c, i) => {
      const cell = el(tag)
      if (b.align[i]) cell.style.textAlign = b.align[i]
      renderInline(c, ctx, cell)
      tr.appendChild(cell)
    })
    into.appendChild(tr)
  }
  const head = el('thead')
  row(b.head, 'th', head)
  const body = el('tbody')
  for (const r of b.rows) row(r, 'td', body)
  t.append(head, body)
  wrap.appendChild(t)
  return wrap
}

export function renderBlocks(list: Block[], ctx: MdContext, into: Node): void {
  for (const b of list) {
    if (b.t === 'p') {
      const p = el('p')
      renderInline(b.c, ctx, p)
      into.appendChild(p)
    } else if (b.t === 'h') {
      const h = el(`h${Math.min(6, b.level + 2)}` as 'h3', 'md-h')
      renderInline(b.c, ctx, h)
      into.appendChild(h)
    } else if (b.t === 'hr') into.appendChild(el('hr'))
    else if (b.t === 'quote') {
      const q = el('blockquote')
      renderBlocks(b.blocks, ctx, q)
      into.appendChild(q)
    } else if (b.t === 'code') into.appendChild(codeBlock(b))
    else if (b.t === 'table') into.appendChild(table(b, ctx))
    else {
      const l = el(b.ordered ? 'ol' : 'ul')
      if (b.ordered && b.start !== 1) (l as HTMLOListElement).start = b.start
      for (const item of b.items) {
        const li = el('li')
        if (item.task !== undefined) {
          li.className = 'md-task'
          const box = el('input')
          box.type = 'checkbox'
          box.disabled = true
          box.checked = item.task
          box.setAttribute('aria-label', item.task ? 'Done' : 'Not done')
          li.appendChild(box)
        }
        renderBlocks(item.blocks, ctx, li)
        l.appendChild(li)
      }
      into.appendChild(l)
    }
  }
}
