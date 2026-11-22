// Chats the server knows (saved and running) and the open message feed of each.
import { REVIEW_REQUEST } from './secondopinion'
import { ChatFeed } from './chatfeed.svelte'
import type { Attachment } from './attachstore.svelte'
import type { SocketLike } from './events'
import { saveQueue, sweepQueues } from './queuestore'

export interface Agent {
  id: string
  name: string
  installed: boolean
  tested: boolean
  missing?: string
  note?: string
}

export interface ChatInfo {
  id: string
  projectId: string
  agent: string
  title: string
  state: 'starting' | 'idle' | 'running' | 'needs_login' | 'stopped'
  error: { code: string; message: string } | null
  attention: boolean
  /** The attention is for the account's usage limit alone; no permission is waiting. */
  limitOnly?: boolean
  mode: string
  modes: { id: string; name: string }[] | null
  model: string
  models: { id: string; name: string }[] | null
  usage: { used: number; size: number; cost?: number; currency?: string } | null
  caps: Record<string, boolean | string[]> | null
  spec?: { folder?: { path?: string; worktree?: boolean }; account?: { id?: string }; filters?: string[]; agentMade?: boolean } | null
  /** Mooring's instructions went to the agent when this chat last started. */
  instructed?: boolean
  /** The group this chat works in, and the group's title when it has one. */
  groupId?: string
  groupTitle?: string
  /** The chat this one started from a summary of. */
  fromChat?: string
  /** After a limit: the account to move to, or null when no other account has room. */
  switchTo?: string | null
  messages: number
  /** An agent reply finished after the user last looked (chat.seen clears it). */
  unseen?: boolean
  /** When the last reply finished, unix ms; 0 for never. */
  finishedAt?: number
  /** When the user last looked, unix ms; 0 for never. */
  seenAt?: number
  /** The end of the newest message of the user or the agent, for lists. */
  lastLine?: string
  updatedAt: number
  /** When the chat was made, unix ms. */
  createdAt?: number
}

type Run = <T>(name: string, args: unknown) => Promise<T>

const socketFor = (id: string) => (): SocketLike =>
  new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/chats/${encodeURIComponent(id)}`) as unknown as SocketLike

class Chats {
  list = $state<Record<string, ChatInfo>>({})
  agents = $state<Agent[]>([])
  /** Starting, by tab id: the command is on its way. */
  starting = $state<Record<string, boolean>>({})
  /** Why the last start of a chat tab failed, by tab id; the start screen shows it when it comes back. */
  startError = $state<Record<string, string>>({})
  /** The first snapshot arrived, so "not in the list" means gone. */
  known = $state(false)
  private feeds = new Map<string, ChatFeed>()
  /** A summary waiting on a new chat's start screen, by tab id: the chat it came from and the editable text. */
  seeds = $state<Record<string, { fromChat: string; text: string; source: string; review?: boolean }>>({})
  /** Unsent text by chat, so a draft survives a tab or project switch. */
  readonly drafts = new Map<string, string>()
  /** Messages written while the agent was replying; each goes out when the reply ends. */
  readonly queued = new Map<string, { text: string; chips: Attachment[] }[]>()
  /** Makes the socket for a chat; tests replace it. */
  connect: (id: string) => () => SocketLike = socketFor

  reset(byProject: Record<string, ChatInfo[]> | undefined, agents: Agent[] | undefined) {
    const next: Record<string, ChatInfo> = {}
    for (const infos of Object.values(byProject ?? {})) for (const i of infos ?? []) next[i.id] = i
    this.list = next
    this.agents = agents ?? []
    this.known = true
    for (const id of [...this.feeds.keys()]) if (!next[id]) this.forget(id)
    if (byProject) sweepQueues((id) => !!next[id])
  }

  /** A project was removed: its chats go with it, and so do their saved queues. */
  dropProject(projectId: string) {
    for (const c of Object.values(this.list)) if (c.projectId === projectId) this.forget(c.id)
  }

  apply(kind: string, data: unknown) {
    const d = data as ChatInfo
    if (kind === 'chat.added' || kind === 'chat.changed') this.list[d.id] = d
    else if (kind === 'chat.removed') this.forget(d.id)
  }

  inProject(projectId: string): ChatInfo[] {
    return Object.values(this.list)
      .filter((c) => c.projectId === projectId)
      .sort((a, b) => b.updatedAt - a.updatedAt)
  }

  /** True while a permission request waits in any chat of the project. */
  attention(projectId: string): boolean {
    return Object.values(this.list).some((c) => c.projectId === projectId && c.attention)
  }

  attentionCount(): number {
    return Object.values(this.list).filter((c) => c.attention).length
  }

  /** The feed for a chat, made on first use and kept so its position survives a tab switch. */
  feed(id: string): ChatFeed {
    let f = this.feeds.get(id)
    if (!f) {
      f = new ChatFeed(this.connect(id), (info) => (this.list[id] = info as ChatInfo))
      this.feeds.set(id, f)
    }
    return f
  }

  private forget(id: string) {
    delete this.list[id]
    this.feeds.get(id)?.shut()
    this.feeds.delete(id)
    this.drafts.delete(id)
    this.queued.delete(id)
    saveQueue(id, [])
  }

  async start(run: Run, projectId: string, id: string, agent: string, worktree?: { branch: string }, toolServers: string[] = [], from?: { fromChat: string; context: string; withChanges?: boolean }) {
    this.starting[id] = true
    delete this.startError[id]
    // Set first: the message box can open before this command answers.
    if (from?.withChanges && !this.drafts.get(id)) this.drafts.set(id, REVIEW_REQUEST)
    try {
      const folder = worktree ? { folder: { worktree: true, ...(worktree.branch ? { branch: worktree.branch } : {}) } } : {}
      const tools = toolServers.length > 0 ? { toolServers: toolServers.map((id) => ({ id })) } : {}
      const spec = worktree || toolServers.length > 0 ? { ...folder, ...tools } : undefined
      this.list[id] = await run<ChatInfo>('chat.start', { id, projectId, agent, ...(spec ? { spec } : {}), ...(from ? from : {}) })
      delete this.seeds[id]
    } catch (e) {
      if (from?.withChanges && this.drafts.get(id) === REVIEW_REQUEST) this.drafts.delete(id)
      this.startError[id] = e instanceof Error && e.message ? e.message : 'Could not start the chat.'
      throw e
    } finally {
      delete this.starting[id]
    }
  }

  async resume(run: Run, id: string) {
    this.list[id] = await run<ChatInfo>('chat.resume', { id })
  }

  /** Stops the agent and keeps the saved chat. */
  async close(run: Run, id: string) {
    this.feeds.get(id)?.shut()
    await run('chat.close', { id })
  }

  async remove(run: Run, id: string) {
    this.forget(id)
    await run('chat.remove', { id })
  }

  /** Turns compaction of long command output on or off for a chat, at once. */
  async setCompact(run: Run, id: string, on: boolean) {
    this.list[id] = await run<ChatInfo>('chat.setFilters', { id, filters: on ? ['compact'] : [] })
  }

  /** Asks for the text a fresh chat can start from. */
  summarize(run: Run, id: string): Promise<{ text: string; source: string }> {
    return run('chat.summarize', { id })
  }

  async refreshAgents(run: Run) {
    this.agents = (await run<{ agents: Agent[] }>('agent.list', {})).agents
  }

  /** Answers a permission request. Returns the option that won, which may be another device's. */
  async answer(run: Run, id: string, requestId: string, optionId: string): Promise<{ accepted: boolean; chosen: string }> {
    const mine = this.feeds.get(id)?.mine
    mine?.add(requestId)
    try {
      const r = await run<{ accepted: boolean; chosen: string }>('chat.permission', { id, requestId, optionId })
      if (!r.accepted) mine?.delete(requestId)
      return r
    } catch (e) {
      mine?.delete(requestId)
      throw e
    }
  }
}

export const chats = new Chats()
