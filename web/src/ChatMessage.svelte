<script lang="ts">
  import ChatChips from './ChatChips.svelte'
  import ChatImages from './ChatImages.svelte'
  import ChatContext from './ChatContext.svelte'
  import ChatTool from './ChatTool.svelte'
  import Markdown from './Markdown.svelte'
  import { answerText, PLAN_STATUS } from './lib/chatfmt'
  import { changedFiles } from './lib/chattool'
  import { lastReplyId } from './lib/chatrelay'
  import { chatPlace, chatRoot } from './lib/chatplace'
  import { actions } from './lib/actions'
  import { chats } from './lib/chatstore.svelte'
  import { app } from './lib/state.svelte'
  import type { ChatFeed } from './lib/chatfeed.svelte'

  let { feed, index, agent, chatId, quietErrors = false }: { feed: ChatFeed; index: number; agent: string; chatId: string; quietErrors?: boolean } = $props()

  // Reads one key of the feed, so a streamed chunk redraws this row and no other.
  const msg = $derived(feed.msgs.get(index))
  const asked = $derived(msg?.kind === 'permission' ? msg : null)
  const chosen = $derived(asked?.options?.find((o) => o.id === asked.chosen))
  // While the banner by the composer asks, the transcript does not repeat the question: not the
  // permission row and not the tool row that is waiting on it.
  const banner = $derived(
    msg?.kind === 'permission'
      ? msg.state === 'pending'
      : msg?.kind === 'tool' && (msg.status ?? 'pending') === 'pending' && feed.pending.some((i) => !!msg.toolId && feed.msgs.get(i)?.toolId === msg.toolId),
  )
  // A chat that needs a sign-in explains it once, in the panel; the agent's raw error is not a second reply.
  const hidden = $derived(banner || (quietErrors && msg?.kind === 'error'))

  const info = $derived(chats.list[chatId])
  const sentImages = $derived(msg?.attachments?.filter((a) => a.kind === 'image' && a.imageId) ?? [])
  const relayFrom = $derived(msg?.relay ? chats.list[msg.relay.chat] : undefined)
  const relayAgent = $derived(msg?.relay ? (chats.agents.find((a) => a.id === msg.relay?.agent)?.name ?? msg.relay.agent) : '')
  // A reply can be sent on once its turn has ended. The button sits on the turn-end row and sends the turn's last reply.
  let relaying = $state(false)
  let relayButton: HTMLButtonElement | undefined = $state()
  function closeRelay(refocus = true) {
    relaying = false
    if (refocus) relayButton?.focus()
  }
  const projectPath = $derived(app.projects.find((p) => p.id === info?.projectId)?.path ?? '')
  const root = $derived(chatRoot(info, projectPath))
  // The last row of a turn says how many files the turn changed and opens them. A turn runs from one
  // message of yours to the next; the one still replying has no end yet.
  const turnEnd = $derived(!!msg && msg.kind !== 'user' && (feed.msgs.get(index + 1)?.kind === 'user' || (index === feed.count - 1 && info?.state !== 'running')))
  const replyId = $derived(turnEnd ? lastReplyId((i) => feed.msgs.get(i), index) : null)
  const changed = $derived.by(() => {
    if (!turnEnd) return []
    const turn = []
    for (let i = index; i >= 0; i--) {
      const m = feed.msgs.get(i)
      if (m?.kind === 'user') break
      turn.push(m)
    }
    return changedFiles(turn, root)
  })
</script>

{#if msg && !hidden}
  <div class="msg {msg.kind}">
    {#if msg.kind === 'user' && msg.job === 'summary'}
      <ChatContext kind="summary" text={msg.text ?? ''} />
    {:else if msg.kind === 'context'}
      <ChatContext kind="context" text={msg.text ?? ''} fromChat={msg.fromChat} changes={msg.changes} />
    {:else if msg.kind === 'user'}
      <p class="who">You</p>
      {#if msg.relay}
        <p class="from">
          From {relayAgent} <span aria-hidden="true">·</span>
          {#if relayFrom}<button type="button" class="link" aria-label="Open the chat {relayFrom.title || 'New chat'}, where this reply came from" onclick={() => actions.openChat(msg.relay!.chat)}>{relayFrom.title || 'New chat'}</button>{:else}<span>a deleted chat</span>{/if}
        </p>
      {/if}
      {#if msg.attachments?.length}<ChatChips chips={msg.attachments} />{/if}
      <p class="text">{msg.text}</p>
      {#if sentImages.length}<ChatImages {chatId} images={sentImages} />{/if}
    {:else if msg.kind === 'agent'}
      <p class="who">{agent}</p>
      <Markdown text={msg.text ?? ''} projectId={info?.projectId ?? ''} />
    {:else if msg.kind === 'thought'}
      <details>
        <summary>Reasoning</summary>
        <p class="text muted">{msg.text}</p>
      </details>
    {:else if msg.kind === 'tool'}
      <ChatTool {msg} {chatId} />
    {:else if asked}
      <p class="ask">
        <span class="label quiet">{answerText(asked.state, chosen)}</span>
        <span class="what">{asked.title}</span>
      </p>
    {:else if msg.kind === 'plan'}
      <p class="who">Plan</p>
      <ol class="plan">
        {#each msg.entries ?? [] as e, i (i)}
          <li class:done={e.status === 'completed'}><span class="pstate">{PLAN_STATUS[e.status] ?? e.status}</span> {e.content}</li>
        {/each}
      </ol>
    {:else if msg.kind === 'notice'}
      <p class="notice">{msg.text}</p>
    {:else if msg.kind === 'error'}
      <p class="error-row"><span class="label">Error</span> {msg.text}</p>
    {/if}
  </div>
{/if}
{#if changed.length || replyId !== null}
  <div class="msg turnend">
    <div class="acts">
      {#if changed.length}
        <button type="button" class="btn small" title={changed.join('\n')} onclick={() => actions.showChanges(info ? chatPlace(chatId, info, projectPath) : undefined)}>
          Changed {changed.length} {changed.length === 1 ? 'file' : 'files'}<span class="sr">. Open the changes</span>
        </button>
      {/if}
      {#if replyId !== null}
        <button type="button" class="btn small" bind:this={relayButton} aria-expanded={relaying} onclick={() => (relaying ? closeRelay() : (relaying = true))}>
          Send to another chat<span class="sr">. Reply from {agent}</span>
        </button>
      {/if}
    </div>
    {#if relaying && replyId !== null}
      {#await import('./ChatRelay.svelte')}
        <p class="from" role="status">Loading…</p>
      {:then r}
        <r.default {chatId} msgId={replyId} onclose={closeRelay} />
      {:catch}
        <p class="error" role="alert">Could not open the panel. Try again.</p>
      {/await}
    {/if}
  </div>
{/if}

<style>
  .msg {
    padding: var(--s-2) max(var(--s-4), calc((100% - var(--chat-col)) / 2));
  }
  .turnend {
    padding-top: 0;
  }
  .who {
    margin: 0 0 var(--s-1);
    color: var(--muted);
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .from {
    overflow-wrap: anywhere;
    margin: 0 0 var(--s-1);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-align: left;
    text-decoration: underline;
  }
  .link:hover {
    color: var(--fg);
  }
  @media (pointer: coarse) {
    .link {
      min-height: 44px;
    }
  }
  .text {
    margin: 0;
    font-size: var(--fs-read);
    line-height: var(--lh-read);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .user .text {
    padding: var(--s-2) var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-m);
    background: var(--surface);
  }
  .muted,
  summary {
    color: var(--muted);
  }
  summary {
    min-height: var(--control-h);
    font-size: var(--fs-s);
    line-height: var(--control-h);
  }
  .ask {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    margin: 0;
    min-height: var(--control-h);
  }
  .label {
    font-weight: var(--fw-semibold);
  }
  .label.quiet {
    color: var(--muted);
  }
  .what {
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .plan {
    margin: 0;
    padding-left: var(--s-5);
  }
  .pstate {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .done {
    color: var(--muted);
  }
  .notice {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .error-row {
    margin: 0;
    color: var(--danger);
    overflow-wrap: anywhere;
  }
</style>
