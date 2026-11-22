<script lang="ts">
  import { onDestroy, tick, untrack } from 'svelte'
  import ChatAccount from './ChatAccount.svelte'
  import { signInAccount } from './lib/accountactions'
  import { terminalTyping } from './lib/termtype'
  import ChatComposer from './ChatComposer.svelte'
  import ChatHeader from './ChatHeader.svelte'
  import ChatMessage from './ChatMessage.svelte'
  import ChatPermission from './ChatPermission.svelte'
  import VirtualList from './VirtualList.svelte'
  import { actions } from './lib/actions'
  import { loginCommand } from './lib/chatfmt'
  import { chats } from './lib/chatstore.svelte'
  import { app } from './lib/state.svelte'

  let { id }: { id: string } = $props()

  const info = $derived(chats.list[id])
  const feed = $derived(info ? chats.feed(id) : null)
  const agent = $derived(chats.agents.find((a) => a.id === info?.agent))
  const agentName = $derived(agent?.name ?? info?.agent ?? 'Agent')
  const command = $derived(loginCommand(agent?.note))
  let follow = $state(true)
  let list: ReturnType<typeof VirtualList> | undefined = $state()
  let resuming = $state(false)
  let resumeError = $state('')
  let tried = false

  // The message socket is open while this view is on screen and the page is visible.
  let visible = $state(!document.hidden)
  const onVisibility = () => (visible = !document.hidden)
  document.addEventListener('visibilitychange', onVisibility)
  onDestroy(() => document.removeEventListener('visibilitychange', onVisibility))

  $effect(() => {
    const f = feed
    if (!f || !visible) return
    return f.open()
  })

  // A chat that was stopped cleanly (a server restart, a closed tab) starts again when shown.
  $effect(() => {
    if (info?.state === 'stopped' && !info.error && !tried) {
      tried = true
      void resume()
    }
  })

  // The composer takes focus the first time the chat is ready.
  let focused = false
  $effect(() => {
    if (info?.state !== 'idle' || focused) return
    focused = true
    void tick().then(() => document.getElementById('msg-' + id)?.focus())
  })

  async function resume() {
    resuming = true
    resumeError = ''
    try {
      await chats.resume((n, a) => app.run(n, a), id)
    } catch (e) {
      resumeError = e instanceof Error ? e.message : 'Could not resume the chat.'
    } finally {
      resuming = false
    }
  }

  // Screen reader line: one sentence when a reply ends, never one per streamed chunk.
  let announce = $state({ n: 0, text: '' })
  let was = ''
  let youStopped = $state(false)
  $effect(() => {
    const now = info?.state ?? ''
    const f = feed
    const before = was
    was = now
    if (!f || before !== 'running' || now === 'running') return
    untrack(() => {
      let last = ''
      for (let i = f.count - 1; i >= 0 && !last; i--) if (f.msgs.get(i)?.kind === 'agent') last = f.msgs.get(i)?.text ?? ''
      const text = youStopped ? `${agentName} stopped replying.` : last ? `${agentName} replied: ${last.slice(0, 400)}` : `${agentName} finished.`
      announce = { n: announce.n + 1, text }
    })
  })

  const noAsk = $derived(!!feed?.noAsk)
  const needsLogin = $derived(info?.state === 'needs_login')
  /** ChatAccount shows the sign-in and the resume button when the chat's account has a sign-in of its own. */
  const ownSignIn = $derived(info ? !!signInAccount(info) : false)
  let copied = $state(false)
  function copyCommand() {
    void navigator.clipboard?.writeText(command).then(() => {
      copied = true
      setTimeout(() => (copied = false), 1500)
    })
  }
  const stopped = $derived(info?.state === 'stopped')
</script>

<div class="chat" data-chat-view={id}>
  {#if !info}
    {#if chats.starting[id]}
      <p class="note" role="status">Starting the chat…</p>
    {:else if !chats.known}
      <p class="note" role="status">Loading chats…</p>
    {:else}
      <!-- The picker loads on its own, so the chat chunk stays under its size limit. -->
      {#await import('./ChatPicker.svelte') then picker}
        <picker.default {id} />
      {:catch}
        <p class="note">The agent list could not load. Reload the page and try again.</p>
      {/await}
    {/if}
  {:else if feed}
    <ChatHeader {id} {info} />
    <ChatAccount {info} agent={agentName} />
    {#if needsLogin && !ownSignIn}
      <section class="panel" aria-label="Sign in needed">
        <p><strong>{agentName} is not signed in.</strong></p>
        {#if info.error?.message && !command && !/not signed in/i.test(info.error.message)}<p>{info.error.message}</p>{/if}
        {#if command}<p>Run <code>{command}</code> in a terminal on the computer that runs Mooring, then resume the chat. Open a terminal types it for you; press Enter to run it.</p>{/if}
        <div class="row">
          <button type="button" class="btn primary" disabled={resuming} onclick={resume}>{resuming ? 'Resuming…' : 'Resume chat'}</button>
          <button type="button" class="btn" onclick={() => (command ? terminalTyping(command) : actions.newTerminal())}>Open a terminal</button>
          {#if command}<button type="button" class="btn" onclick={copyCommand}>{copied ? 'Copied' : 'Copy command'}</button>{/if}
        </div>
        {#if resumeError}<p class="error" role="alert">{resumeError}</p>{/if}
      </section>
    {:else if stopped}
      <section class="panel" aria-label="Chat stopped">
        <p>
          <strong>{resuming ? 'Starting the agent again…' : `${agentName} is not running.`}</strong>
          {info.error?.message ?? 'The messages are saved. Resume to keep going.'}
        </p>
        <div class="row">
          <button type="button" class="btn primary" disabled={resuming} onclick={resume}>{resuming ? 'Resuming…' : 'Resume chat'}</button>
        </div>
        {#if resumeError}<p class="error" role="alert">{resumeError}</p>{/if}
      </section>
    {/if}
    {#if noAsk}
      <p class="warn"><span class="dot" aria-hidden="true"></span>{agentName} did not switch to asking before it acts, so it may run commands without asking. Pick a mode that asks, if the agent has one.</p>
    {/if}
    {#if feed.wire === 'reconnecting'}<p class="status" role="status">Reconnecting to the chat…</p>{/if}
    {#if feed.count === 0}
      <p class="note" role="status">{feed.wire === 'connecting' ? 'Loading messages…' : 'No messages yet. Write the first one below.'}</p>
      <div class="fill"></div>
    {:else}
      <VirtualList bind:this={list} bind:follow count={feed.count} label={`${info.title || 'Chat'} messages`}>
        {#snippet row(i: number)}
          <ChatMessage {feed} index={i} agent={agentName} chatId={id} quietErrors={needsLogin} />
        {/snippet}
      </VirtualList>
    {/if}
    <ChatPermission {id} {feed} folder={info.spec?.folder?.path || app.projects.find((p) => p.id === info.projectId)?.path || ''} />
    <ChatComposer {id} {info} agent={agentName} onSent={() => ((youStopped = false), untrack(() => list?.toBottom()))} onStopped={() => (youStopped = true)} />
    {#key announce.n}<p class="sr" role="log" aria-live="polite">{announce.text}</p>{/key}
  {/if}
</div>

<style>
  .chat {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--bg);
  }
  .note,
  .status {
    margin: 0;
    padding: var(--s-3) max(var(--s-4), calc((100% - var(--chat-col)) / 2));
    color: var(--muted);
  }
  .status {
    padding: var(--s-1) max(var(--s-4), calc((100% - var(--chat-col)) / 2));
    font-size: var(--fs-s);
  }
  .warn {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin: 0;
    padding: var(--s-2) max(var(--s-4), calc((100% - var(--chat-col)) / 2));
    background: var(--attention-bg);
    color: var(--attention);
    font-weight: 600;
  }
  .dot {
    flex: none;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--attention);
  }
  .fill {
    flex: 1;
  }
  .panel {
    display: grid;
    justify-items: start;
    gap: var(--s-2);
    padding: var(--s-3) max(var(--s-4), calc((100% - var(--chat-col)) / 2));
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .panel p {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
</style>
