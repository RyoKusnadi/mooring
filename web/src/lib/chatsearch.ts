// Searching every saved chat, in every project: GET /api/chats/search.
import type { Fetcher } from './api'
import { getJSON, qs } from './filesapi'

export interface ChatHit {
  chatId: string
  projectId: string
  title: string
  agent: string
  state: string
  updatedAt: number
  /** The best matching passage; the server puts each hit between \x02 and \x03. */
  snippet: string
}

export interface Piece {
  text: string
  hit: boolean
}

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export async function searchChats(q: string, f: Fetcher = browserFetch, signal?: AbortSignal): Promise<ChatHit[]> {
  const v = await getJSON<{ hits: ChatHit[] }>(`/api/chats/search?${qs({ q, limit: 50 })}`, f, signal)
  return v.hits
}

/** Splits a snippet into plain and matching pieces, so it can be shown without HTML. */
export function pieces(snippet: string): Piece[] {
  const out: Piece[] = []
  for (const part of snippet.split('\x02')) {
    const end = part.indexOf('\x03')
    if (end < 0) {
      if (part) out.push({ text: part, hit: false })
      continue
    }
    if (end > 0) out.push({ text: part.slice(0, end), hit: true })
    if (end + 1 < part.length) out.push({ text: part.slice(end + 1), hit: false })
  }
  return out
}

/** Groups hits by project, keeping the order of the first hit of each project. */
export function byProject(hits: ChatHit[]): { projectId: string; hits: ChatHit[] }[] {
  const groups = new Map<string, ChatHit[]>()
  for (const h of hits) {
    const g = groups.get(h.projectId)
    if (g) g.push(h)
    else groups.set(h.projectId, [h])
  }
  return [...groups].map(([projectId, list]) => ({ projectId, hits: list }))
}
