// Key names for the screen, and the browser-style chords (Cmd on a Mac, Ctrl elsewhere).
export const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)

const MAC_KEYS: Record<string, string> = { 'Alt+': '⌥', 'Shift+': '⇧', 'Ctrl+': '⌃', Left: '←', Right: '→', Up: '↑', Down: '↓' }

/** "Alt+Shift+Right" reads "⌥⇧→" on a Mac and stays as written elsewhere. */
export function showKey(text: string, mac = isMac): string {
  if (!mac) return text
  let out = text
  for (const [k, v] of Object.entries(MAC_KEYS)) out = out.replaceAll(k, v)
  return out
}

export type Chord = 'file' | 'palette' | 'search' | 'settings' | 'termfind'

type KeyLike = Pick<KeyboardEvent, 'code' | 'metaKey' | 'ctrlKey' | 'altKey' | 'shiftKey'> & { isComposing?: boolean; keyCode?: number }

/** A key that belongs to an input method (IME) composition: never a shortcut. */
export const composing = (e: { isComposing?: boolean; keyCode?: number }) => !!e.isComposing || e.keyCode === 229

/**
 * Cmd+P, Cmd+Shift+P, Cmd+Shift+F and Cmd+, (Ctrl on other systems). In a terminal the find-in-output key
 * replaces the project search chord on systems without Cmd: Cmd+F on a Mac, Ctrl+Shift+F elsewhere. Cmd+W and Cmd+T are never read:
 * they belong to the browser. On a Mac they work in the editor and in a terminal too, because
 * shells and CodeMirror do not use Cmd. Elsewhere Ctrl+P is a shell's "previous line", so it is
 * left to a focused terminal (`inTerminal`).
 */
export function chordOf(e: KeyLike, inTerminal = false, mac = isMac): Chord | null {
  if (composing(e) || e.altKey || (mac ? !e.metaKey || e.ctrlKey : !e.ctrlKey || e.metaKey)) return null
  if (e.code === 'KeyP') return e.shiftKey ? 'palette' : inTerminal && !mac ? null : 'file'
  if (e.code === 'KeyF') {
    // In a terminal: Cmd+F on a Mac, Ctrl+Shift+F elsewhere (plain Ctrl+F is the shell's "forward one character").
    if (inTerminal && e.shiftKey !== mac) return 'termfind'
    if (e.shiftKey) return 'search'
  }
  if (e.code === 'Comma' && !e.shiftKey) return inTerminal && !mac ? null : 'settings'
  return null
}

/** The modifier of the browser-style chords as shown on screen: "⌘" or "Ctrl+". */
export const modLabel = isMac ? '⌘' : 'Ctrl+'
