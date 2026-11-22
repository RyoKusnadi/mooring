// Attention outside the tab: "(2) Mooring" as the page title and an amber dot on the favicon.
const BASE = 'Mooring'

export function titleFor(n: number): string {
  return n > 0 ? `(${n}) ${BASE}` : BASE
}

/** A small teal square with an M; with attention an amber dot sits in the corner. Colours match the light tokens. */
export function faviconFor(n: number): string {
  const dot = n > 0 ? '<circle cx="24" cy="8" r="7" fill="#8a5a00" stroke="#f9fbf9" stroke-width="2"/>' : ''
  const svg =
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#1d6b66"/>' +
    '<path d="M8 23V10l8 8 8-8v13" fill="none" stroke="#fff" stroke-width="3" stroke-linejoin="round"/>' +
    dot +
    '</svg>'
  return 'data:image/svg+xml,' + encodeURIComponent(svg)
}

/** Chats plus terminals that need the user. */
export function needCount(chatsWaiting: number, terminalsWaiting: number): number {
  return chatsWaiting + terminalsWaiting
}

/** The status bar line when a terminal needs you; with no terminal, `chatText` is kept as it is. */
export function needText(chatsWaiting: number, terminalsWaiting: number, chatText: string): string {
  if (terminalsWaiting === 0) return chatText
  const t = terminalsWaiting === 1 ? '1 terminal' : `${terminalsWaiting} terminals`
  if (chatsWaiting === 0) return terminalsWaiting === 1 ? `${t} needs you` : `${t} need you`
  const c = chatsWaiting === 1 ? '1 chat' : `${chatsWaiting} chats`
  return `${c} and ${t} need you`
}

const written = new WeakMap<object, number>()

/** Writes the title and the favicon link for `n` chats waiting on the user. */
export function showAttention(n: number, doc: Pick<Document, 'title' | 'head' | 'querySelector' | 'createElement'> = document): void {
  doc.title = titleFor(n)
  let link = doc.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = doc.createElement('link')
    link.rel = 'icon'
    link.type = 'image/svg+xml'
    doc.head.appendChild(link)
  }
  if (written.get(link) !== n) link.href = faviconFor(n)
  written.set(link, n)
}
