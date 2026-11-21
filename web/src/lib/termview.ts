// xterm.js for one terminal. This file and what it imports load on first use of a terminal
// (a dynamic import in Terminal.svelte), so none of it is in the app shell.
import { FitAddon } from '@xterm/addon-fit'
import { termFontSize } from './termfont'
import { WebglAddon } from '@xterm/addon-webgl'
import { Terminal, type ITheme } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { isShortcut } from './actions'
import { terms } from './termstore.svelte'
import { TerminalConnection, type StartInfo, type TermSocket } from './termwire'

const FIT_DELAY_MS = 80
const views = new Map<string, TermView>()
let current: TermView | null = null

function theme(): ITheme {
  const cs = getComputedStyle(document.documentElement)
  const v = (name: string) => cs.getPropertyValue(name).trim()
  const t: Record<string, string> = {
    background: v('--bg'),
    foreground: v('--fg'),
    cursor: v('--accent'),
    cursorAccent: v('--bg'),
    selectionBackground: v('--selected'),
  }
  const names = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white']
  names.forEach((n, i) => {
    t[n] = v(`--ansi-${i}`)
    t['bright' + n[0].toUpperCase() + n.slice(1)] = v(`--ansi-${i + 8}`)
  })
  return t
}

const media = matchMedia('(prefers-color-scheme: dark)')
const retheme = () => views.forEach((v) => v.restyle())
media.addEventListener('change', retheme)
window.addEventListener('mooring-theme', retheme)

class TermView {
  readonly term: Terminal
  private fit = new FitAddon()
  private conn: TerminalConnection
  private stage: HTMLElement | null = null
  private box: HTMLElement | null = null
  private observer: ResizeObserver | null = null
  private opened = false
  private fitTimer: ReturnType<typeof setTimeout> | undefined
  private initialFit = true
  private pending = 0
  private cols = 80
  private rows = 24
  private visible = true

  constructor(readonly id: string) {
    const info = terms.list[id]
    this.term = new Terminal({
      cols: info?.cols ?? 80,
      rows: info?.rows ?? 24,
      fontFamily: getComputedStyle(document.documentElement).getPropertyValue('--font-mono'),
      fontSize: termFontSize(document.documentElement.dataset.size),
      scrollback: 5000,
      cursorBlink: false,
      allowProposedApi: false,
      // Tools that pick their own colours (Claude Code draws for a dark screen by default) stay readable on
      // either theme: xterm lifts any text below 4.5:1 against the cell behind it.
      minimumContrastRatio: 4.5,
      theme: theme(),
    })
    this.term.loadAddon(this.fit)
    const url = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/terminals/${encodeURIComponent(id)}`
    this.conn = new TerminalConnection({
      createSocket: () => new WebSocket(url) as unknown as TermSocket,
      onStart: (s) => this.start(s),
      onData: (bytes, pos) => this.output(bytes, pos),
      onSize: (c, r) => this.serverSize(c, r),
      onExit: () => terms.patch(id, { wire: 'ended' }),
      onState: (wire) => terms.patch(id, { wire }),
    })
    this.term.onTitleChange((t) => terms.patch(id, { title: t }))
    this.term.onData((d) => this.input(d))
    this.term.onBinary((d) => this.conn.sendInput(Uint8Array.from(d, (c) => c.charCodeAt(0))))
    this.term.attachCustomKeyEventHandler((e) => !isShortcut(e))
    terms.views.set(id, this)
  }

  /** Applies the theme and the text size, then refits so the shell learns the new columns. */
  restyle() {
    this.term.options.theme = theme()
    const size = termFontSize(document.documentElement.dataset.size)
    if (this.term.options.fontSize !== size) {
      this.term.options.fontSize = size
      if (this.opened) this.scheduleFit()
    }
  }

  /** Puts the terminal into `stage` and starts (or resumes) the connection. `box` is what sizes it. */
  mount(stage: HTMLElement, box: HTMLElement) {
    this.stage = stage
    this.box = box
    if (!this.opened) {
      this.term.open(stage)
      this.opened = true
      try {
        const gl = new WebglAddon()
        gl.onContextLoss(() => gl.dispose())
        this.term.loadAddon(gl)
      } catch {
        // No WebGL here: xterm keeps its DOM renderer.
      }
    } else if (this.term.element) {
      stage.appendChild(this.term.element)
      this.term.refresh(0, this.term.rows - 1)
    }
    this.initialFit = true
    this.observer = new ResizeObserver(() => this.scheduleFit())
    this.observer.observe(box)
    current = this
    this.setVisible(!document.hidden)
  }

  /** The tab is hidden or gone from the page: stop the socket, keep the screen and the position. */
  unmount() {
    this.observer?.disconnect()
    this.observer = null
    clearTimeout(this.fitTimer)
    this.conn.stop()
    this.stage = this.box = null
    if (current === this) current = null
  }

  /** The browser tab is shown or hidden. A hidden terminal draws nothing and keeps its place. */
  setVisible(visible: boolean) {
    this.visible = visible
    if (!this.box) return
    if (!visible) return this.conn.stop()
    this.localFit()
    this.conn.start()
  }

  focus() {
    this.term.focus()
  }

  selection(): string {
    return this.term.getSelection()
  }

  /** The shell is gone: stop taking keys so Tab can leave the screen. */
  end() {
    this.term.options.disableStdin = true
    this.term.blur()
  }

  /** Keys from the on-screen row. */
  press(data: string) {
    this.input(data)
    this.term.focus()
  }

  dispose() {
    this.unmount()
    this.term.dispose()
    views.delete(this.id)
    terms.views.delete(this.id)
  }

  private localFit(): { cols: number; rows: number } | null {
    const d = this.opened ? this.fit.proposeDimensions() : undefined
    if (!d || !(d.cols >= 2) || !(d.rows >= 1)) return null // hidden or not laid out yet
    return { cols: d.cols, rows: d.rows }
  }

  private differs(d: { cols: number; rows: number } | null): d is { cols: number; rows: number } {
    return !!d && (d.cols !== this.cols || d.rows !== this.rows)
  }

  private scheduleFit() {
    clearTimeout(this.fitTimer)
    this.fitTimer = setTimeout(() => {
      const first = this.initialFit
      this.initialFit = false
      this.showLetterbox()
      // The last window to change size sets the PTY size; the first look at a joined terminal does not.
      if (!first) this.takeSize()
    }, FIT_DELAY_MS)
  }

  /** Asks the server for the size that fits this window. */
  private takeSize() {
    const d = this.localFit()
    if (this.differs(d)) this.conn.sendResize(d.cols, d.rows)
  }

  private showLetterbox() {
    const d = this.localFit()
    terms.patch(this.id, { letterbox: this.differs(d) ? { cols: this.cols, rows: this.rows } : null })
  }

  private serverSize(cols: number, rows: number) {
    this.cols = cols
    this.rows = rows
    if (this.term.cols !== cols || this.term.rows !== rows) this.term.resize(cols, rows)
    this.showLetterbox()
  }

  private start(s: StartInfo) {
    if (s.reset) {
      this.term.reset()
      this.pending = 0
    }
    this.serverSize(s.cols, s.rows)
    if (s.reset && s.modes) this.term.write(s.modes)
    // Alone on a terminal (the first window, or a reload): fit to this window.
    if (s.hello && s.others === 0) this.takeSize()
  }

  private output(bytes: Uint8Array, pos: number) {
    this.pending++
    this.term.write(bytes, () => {
      this.pending--
      this.conn.drawn(pos + bytes.length, this.pending === 0)
    })
  }

  private input(data: string) {
    const ui = terms.ui[this.id]
    if (ui?.ctrl) {
      terms.patch(this.id, { ctrl: false })
      if (data.length === 1) data = String.fromCharCode(data.toUpperCase().charCodeAt(0) & 0x1f)
    }
    this.takeSize() // typing makes this window the one that sets the size
    this.conn.sendInput(data)
  }
}

export function openView(id: string): TermView {
  let v = views.get(id)
  if (!v) {
    v = new TermView(id)
    views.set(id, v)
  }
  return v
}

export type { TermView }

// A read-only window onto the screen for browser tests, only when the page URL has ?test.
if (new URLSearchParams(location.search).has('test')) {
  const screen = (id?: string) => {
    const v = (id && views.get(id)) || current
    if (!v) return null
    const b = v.term.buffer.active
    const rows = Array.from({ length: v.term.rows }, (_, i) => b.getLine(b.viewportY + i)?.translateToString(true) ?? '')
    return { rows, cols: v.term.cols, height: v.term.rows, alt: b.type === 'alternate', cursor: [b.cursorX, b.cursorY] }
  }
  ;(window as unknown as { __mooringTest: unknown }).__mooringTest = { screen }
}
