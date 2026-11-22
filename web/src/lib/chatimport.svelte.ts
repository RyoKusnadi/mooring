// Bringing in a chat that was started in an agent's own program: the agent lists its sessions for
// this project, and one of them becomes a Mooring chat.
import { chats, type ChatInfo } from './chatstore.svelte'

type Run = <T>(name: string, args: unknown) => Promise<T>

export interface AgentSession {
  sessionId: string
  title: string
  updatedAt: string
}

/** A name for a session: its title, or "Untitled session" and the start of its id. */
export const sessionName = (s: AgentSession): string => s.title.trim() || `Untitled session ${s.sessionId.slice(0, 7)}`

/** Unix seconds of an ISO time, or null when the agent sent none or something unreadable. */
export function sessionTime(s: AgentSession): number | null {
  const t = Date.parse(s.updatedAt)
  return Number.isNaN(t) ? null : Math.floor(t / 1000)
}

class ChatImport {
  /** The palette asked for this; the next new-chat tab opens with the import section showing. */
  wanted = $state(false)
  agent = $state('')
  sessions = $state<AgentSession[] | null>(null)
  loading = $state(false)
  error = $state('')
  importing = $state('')
  private seq = 0

  reset() {
    this.seq++
    this.sessions = null
    this.loading = false
    this.error = ''
    this.importing = ''
  }

  async find(run: Run, projectId: string, agent: string) {
    const mine = ++this.seq
    this.sessions = null
    this.error = ''
    this.loading = true
    try {
      const r = await run<{ sessions: AgentSession[] | null }>('chat.agentSessions', { projectId, agent })
      if (mine !== this.seq) return
      this.sessions = [...(r.sessions ?? [])].sort((a, b) => (sessionTime(b) ?? 0) - (sessionTime(a) ?? 0))
    } catch (e) {
      if (mine === this.seq) this.error = e instanceof Error && e.message ? e.message : 'Could not list the sessions.'
    } finally {
      if (mine === this.seq) this.loading = false
    }
  }

  /** `id` is the new-chat tab's id, which the chat takes over. */
  async bring(run: Run, projectId: string, id: string, agent: string, s: AgentSession) {
    this.importing = s.sessionId
    this.error = ''
    try {
      chats.list[id] = await run<ChatInfo>('chat.import', { id, projectId, agent, sessionId: s.sessionId, title: s.title })
    } catch (e) {
      this.error = e instanceof Error && e.message ? e.message : 'Could not bring in that chat.'
    } finally {
      this.importing = ''
    }
  }
}

export const chatImport = new ChatImport()
