// What the next message of a chat carries along: lines from the editor and saved command
// outputs. Kept here, not in the composer,
// so a selection sent from a file tab waits for a chat tab that is not shown yet.
import { actions } from './actions'
import { chats } from './chatstore.svelte'
import { chatTabs, pickChatTab } from './chatpick'
import { app } from './state.svelte'

export const MAX_ATTACHMENTS = 8
export const MAX_ATTACH_BYTES = 64 * 1024

export type Attachment =
  | { kind: 'selection'; path: string; from: number; to: number; text: string }
  | { kind: 'output'; outputId: string; lines: number }

class AttachStore {
  pending = $state<Record<string, Attachment[]>>({})
  /** Terminal text waiting to be added to a chat's draft; the composer takes it. */
  texts = $state<Record<string, string>>({})

  list(chatId: string): Attachment[] {
    return this.pending[chatId] ?? []
  }

  /** Adds one, or replaces the same lines or output. Returns an error text when it cannot. */
  add(chatId: string, a: Attachment): string {
    const now = this.list(chatId).filter((x) => !same(x, a))
    if (now.length >= MAX_ATTACHMENTS) return `A message takes at most ${MAX_ATTACHMENTS} attachments. Remove one first.`
    const bytes = [...now, a].reduce((n, x) => n + (x.kind === 'selection' ? new TextEncoder().encode(x.text).length : 0), 0)
    if (bytes > MAX_ATTACH_BYTES) return 'The attached lines are over 64 KB together. Select fewer lines.'
    this.pending[chatId] = [...now, a]
    return ''
  }

  remove(chatId: string, index: number) {
    this.pending[chatId] = this.list(chatId).filter((_, i) => i !== index)
  }

  clear(chatId: string) {
    delete this.pending[chatId]
  }

  addText(chatId: string, text: string) {
    this.texts[chatId] = (this.texts[chatId] ? this.texts[chatId] + '\n\n' : '') + text
  }

  takeText(chatId: string): string {
    const t = this.texts[chatId] ?? ''
    delete this.texts[chatId]
    return t
  }

  /** The chat that gets what the user sends from a file or terminal: the one in the focused pane, else the first. */
  target(): string | null {
    const l = app.layout
    if (!l) return null
    return pickChatTab(l, (id) => !!chats.list[id], (id) => chats.list[id]?.state === 'idle')
  }

  /** Set while more than one chat could take what was sent: the dialog lists them, and `send` finishes the job. */
  asking = $state<{ ids: string[]; send: (id: string) => string } | null>(null)

  /** Sends to the only chat, or asks which one when several are open. */
  private deliver(none: string, send: (id: string) => string): string {
    if (this.asking) return 'Choose a chat for the first send before sending again.'
    const l = app.layout
    const ids = l ? chatTabs(l, (id) => !!chats.list[id]) : []
    if (ids.length > 1) {
      this.asking = { ids, send }
      return ''
    }
    const id = this.target()
    return id ? send(id) : none
  }

  /** Adds lines from the editor to the chosen chat and shows that chat. Returns an error text when it cannot. */
  sendSelection(a: Attachment): string {
    return this.deliver('No chat is open in this project. Start one, then send the lines again.', (id) => {
      const err = this.add(id, a)
      if (!err) actions.focusTab(id)
      return err
    })
  }

  /** Puts the comments on diff lines into a chat's draft. `sent` runs once they are in, which may be after the chat is picked. */
  sendComments(text: string, sent: () => void): string {
    if (new TextEncoder().encode(text).length > MAX_ATTACH_BYTES) return 'The comments are over 64 KB. Send fewer.'
    return this.deliver('No chat is open in this project. Start one, then send the comments again.', (id) => {
      this.addText(id, text)
      sent()
      actions.focusTab(id)
      return ''
    })
  }

  sendText(text: string): string {
    if (new TextEncoder().encode(text).length > MAX_ATTACH_BYTES) return 'The selection is over 64 KB. Select less.'
    return this.deliver('No chat is open in this project. Start one, then send the selection again.', (id) => {
      this.addText(id, '```\n' + text.replace(/\n+$/, '') + '\n```')
      actions.focusTab(id)
      return ''
    })
  }
}

function same(a: Attachment, b: Attachment): boolean {
  if (a.kind === 'output' && b.kind === 'output') return a.outputId === b.outputId
  return a.kind === 'selection' && b.kind === 'selection' && a.path === b.path && a.from === b.from && a.to === b.to
}

/** What `chat.prompt` takes: the text of a selection is sent, an output goes by id. */
export function wire(list: Attachment[]): unknown[] {
  return list.map((a) => (a.kind === 'output' ? { kind: 'output', outputId: a.outputId } : a))
}

export const attach = new AttachStore()
