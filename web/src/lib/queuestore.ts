// Messages queued behind a running reply, kept per chat in this browser so a reload does not drop
// them. Only the typed text is kept: attachments hold lines of files and command output, and stay
// in memory. The size is capped, so a long queue cannot fill the browser's storage.
//
// An entry belongs to the browser tab that wrote it (a per-tab id in sessionStorage, which a reload
// keeps). Only that tab restores it, so two tabs on the same chat never both send it.

const PREFIX = 'mooring.queue.'
const TAB_KEY = 'mooring.tab'
export const MAX_QUEUED = 10
export const MAX_TOTAL_CHARS = 60_000
/** All chats together. When it is passed, the entries written longest ago go first. */
export const MAX_ALL_CHARS = 200_000

const store = (): Storage | null => {
  try {
    return localStorage
  } catch {
    return null
  }
}

/** The texts that fit the caps, in order; the first one that would not fit ends the list. */
export function withinCaps(texts: string[]): string[] {
  const out: string[] = []
  let total = 0
  for (const t of texts) {
    if (out.length >= MAX_QUEUED || total + t.length > MAX_TOTAL_CHARS) break
    out.push(t)
    total += t.length
  }
  return out
}

/** This tab's id: new for a new tab, the same after a reload. */
function tabId(): string {
  try {
    let id = sessionStorage.getItem(TAB_KEY)
    if (!id) {
      id = Math.random().toString(36).slice(2)
      sessionStorage.setItem(TAB_KEY, id)
    }
    return id
  } catch {
    return 'none'
  }
}

interface Entry {
  tab: string
  at: number
  texts: string[]
}

function read(key: string): Entry | null {
  try {
    const raw = store()?.getItem(key)
    const e: unknown = raw ? JSON.parse(raw) : null
    if (!e || typeof e !== 'object' || !Array.isArray((e as Entry).texts)) return null
    const { tab, at, texts } = e as Entry
    return { tab: String(tab), at: Number(at) || 0, texts: texts.filter((t): t is string => typeof t === 'string' && t.trim() !== '') }
  } catch {
    return null
  }
}

const size = (e: Entry | null) => (e ? e.texts.reduce((n, t) => n + t.length, 0) : 0)

/** The queue this tab saved for the chat, if any. */
export function loadQueue(chatId: string): string[] {
  const e = read(PREFIX + chatId)
  return e && e.tab === tabId() ? withinCaps(e.texts) : []
}

function keysOf(): string[] {
  const s = store()
  if (!s) return []
  return Array.from({ length: s.length }, (_, i) => s.key(i) ?? '').filter((k) => k.startsWith(PREFIX))
}

/** Stores the texts, or forgets the chat's entry when there are none. */
export function saveQueue(chatId: string, texts: string[]) {
  try {
    const kept = withinCaps(texts)
    if (!kept.length) return void store()?.removeItem(PREFIX + chatId)
    store()?.setItem(PREFIX + chatId, JSON.stringify({ tab: tabId(), at: Date.now(), texts: kept } satisfies Entry))
    trimAll()
  } catch {
    // Storage is full or off: the queue still works until the page reloads.
  }
}

/** Drops the oldest entries until every chat's queue together is within the global cap. */
function trimAll() {
  const entries = keysOf().map((k) => ({ k, e: read(k) }))
  let total = entries.reduce((n, x) => n + size(x.e), 0)
  for (const x of entries.sort((a, b) => (a.e?.at ?? 0) - (b.e?.at ?? 0))) {
    if (total <= MAX_ALL_CHARS) return
    total -= size(x.e)
    store()?.removeItem(x.k)
  }
}

/** Forgets the entries of chats that no longer exist. Call it once the chat list has loaded. */
export function sweepQueues(live: (chatId: string) => boolean) {
  try {
    for (const k of keysOf()) if (!live(k.slice(PREFIX.length))) store()?.removeItem(k)
  } catch {
    // Storage is off.
  }
}
