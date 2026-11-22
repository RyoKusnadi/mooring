// Tab titles. A terminal tab shows what its shell reports, a chat tab the chat's title, else the stored
// title, with the host's name in front on a remote tab, and a number when two tabs in one pane would read the same.
export interface LabelTab {
  id: string
  kind: string
  title: string
  host?: string
}

export function tabLabels(tabs: LabelTab[], liveTitle: (id: string, kind: string) => string | undefined, hostName: (host: string) => string | undefined = () => undefined): Map<string, string> {
  const named = (t: LabelTab) => liveTitle(t.id, t.kind)?.trim() || (t.kind === 'chat' ? 'New chat' : t.title || 'Blank')
  // A tab on another computer always says which one: a shell's own title would not.
  const base = (t: LabelTab) => {
    const b = named(t)
    const h = t.host ? hostName(t.host) : undefined
    return h && !b.includes(h) ? `${h}: ${b}` : b
  }
  const seen = new Map<string, number>()
  const out = new Map<string, string>()
  for (const t of tabs) {
    const b = base(t)
    const n = (seen.get(b) ?? 0) + 1
    seen.set(b, n)
    out.set(t.id, n > 1 ? `${b} ${n}` : b)
  }
  return out
}
