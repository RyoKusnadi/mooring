<script lang="ts">
  // One chat on the status board: what it is, how long ago, and what can be done without opening it.
  import AttentionMark from './AttentionMark.svelte'
  import ChatPermission from './ChatPermission.svelte'
  import { chatState } from './lib/boardfmt'
  import type { GroupKey } from './lib/board'
  import { chats, type ChatInfo } from './lib/chatstore.svelte'
  import { relativeTime } from './lib/historyapi'
  import { app } from './lib/state.svelte'

  let { chat, group, project, agent, now, onOpen }: { chat: ChatInfo; group: GroupKey; project: string; agent: string; now: number; onOpen: () => void } = $props()

  const when = $derived(group === 'finished' ? (chat.finishedAt || chat.updatedAt) : chat.updatedAt)
  const note = $derived(chatState(chat, group))
  // A permission prompt needs the chat's messages, so a waiting row holds the chat's socket.
  const feed = $derived(group === 'waiting' ? chats.feed(chat.id) : null)
  $effect(() => {
    const f = feed
    if (!f) return
    return f.open()
  })

  let text = $state('')
  let sending = $state(false)
  let error = $state('')
  const canReply = $derived(chat.state === 'idle' && (group === 'finished' || group === 'idle'))

  async function send(e: SubmitEvent) {
    e.preventDefault()
    const body = text.trim()
    if (!body || sending) return
    sending = true
    error = ''
    try {
      await app.run('chat.prompt', { id: chat.id, text: body })
      text = ''
    } catch (err) {
      error = err instanceof Error && err.message ? err.message : 'Could not send the reply.'
    } finally {
      sending = false
    }
  }
</script>

<li class={group} data-chat={chat.id}>
  <button type="button" class="open" onclick={onOpen}>
    <span class="title">{#if group === 'waiting' && !chat.limitOnly}<span aria-hidden="true"><AttentionMark /></span>{/if}<span class="t" title={chat.title || 'New chat'}>{chat.title || 'New chat'}</span></span>
    <span class="meta" title={project}>{note ? `${note} · ` : ''}{relativeTime(when / 1000, now)} · {agent} · {project}</span>
  </button>
  {#if feed}
    <ChatPermission id={chat.id} {feed} stacked />
  {/if}
  {#if canReply}
    <form onsubmit={send}>
      <input class="input" type="text" aria-label={`Reply to ${agent}, ${chat.title || 'New chat'}`} placeholder={`Reply to ${agent}`} autocomplete="off" maxlength="20000" bind:value={text} />
      <button type="submit" class="btn" disabled={sending || !text.trim()}>Send</button>
    </form>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  {/if}
</li>

<style>
  li {
    display: grid;
    gap: var(--s-1);
    padding: var(--s-0) 0 var(--s-1);
    border-radius: var(--r-s);
  }
  .open {
    display: grid;
    gap: var(--s-0);
    width: 100%;
    min-height: var(--control-h);
    padding: var(--s-1) var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .open:hover {
    background: var(--hover);
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
  }
  .t {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta {
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  form {
    display: flex;
    gap: var(--s-1);
    padding: 0 var(--s-2);
  }
  .input {
    flex: 1;
    min-width: 0;
  }
  .error {
    margin: 0;
    padding: 0 var(--s-2);
    color: var(--danger);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
</style>
