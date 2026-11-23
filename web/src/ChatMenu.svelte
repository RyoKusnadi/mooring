<script lang="ts">
  // The "…" of the chat header: shorten long output, a new chat from a summary, context and cost.
  import { tick } from 'svelte'
  import ChatGroupForm from './ChatGroupForm.svelte'
  import Icon from './Icon.svelte'
  import { actions } from './lib/actions'
  import { chats, type ChatInfo } from './lib/chatstore.svelte'
  import { canSummarize, tokens } from './lib/chatfmt'
  import { costText, summarizeError } from './lib/tokenfmt'
  import { chatUsage } from './lib/chatusage'
  import { app } from './lib/state.svelte'

  let { id, info }: { id: string; info: ChatInfo } = $props()

  let shown = $state(false)
  let error = $state('')
  let busy = $state<'' | 'summary' | 'review'>('')
  let root: HTMLElement | undefined = $state()
  let trigger: HTMLButtonElement | undefined = $state()
  let panel: HTMLElement | undefined = $state()
  const uid = $props.id()

  const on = $derived(!!info.spec?.filters?.includes('compact'))
  const usable = $derived(canSummarize(info))
  const usage = $derived(info.usage && info.usage.size > 0 ? info.usage : null)
  const percent = $derived(usage ? Math.min(100, Math.round((usage.used / usage.size) * 100)) : 0)
  const near = $derived(percent >= 80)
  const plan = $derived(chatUsage(info))

  async function open() {
    shown = true
    await tick()
    panel?.querySelector<HTMLElement>('input, button:not(:disabled)')?.focus()
  }
  function close(refocus = true) {
    shown = false
    if (refocus) trigger?.focus()
  }

  async function toggle(want: boolean) {
    error = ''
    try {
      await chats.setCompact((n, a) => app.run(n, a), id, want)
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not change that.'
    }
  }

  async function summarize(review = false) {
    error = ''
    busy = review ? 'review' : 'summary'
    try {
      const r = await chats.summarize((n, a) => app.run(n, a), id)
      close(false)
      actions.chatFromSummary(id, r.text, r.source, review)
    } catch (e) {
      error = summarizeError(e)
    } finally {
      busy = ''
    }
  }
</script>

<svelte:window
  onpointerdown={(e) => shown && !root?.contains(e.target as Node) && close(false)}
  onkeydown={(e) => shown && e.key === 'Escape' && (e.stopPropagation(), close())}
/>

<span class="wrap" bind:this={root}>
  <button
    type="button"
    class="btn ghost icon"
    class:near
    bind:this={trigger}
    aria-label={near ? `Chat options. Context is ${percent}% full` : 'Chat options'}
    title="Chat options"
    aria-expanded={shown}
    aria-controls="{uid}-p"
    onclick={() => (shown ? close() : open())}
  ><Icon name="more" />{#if near}<span class="dot" aria-hidden="true"></span>{/if}</button>
  {#if shown}
    <div class="panel" id="{uid}-p" role="group" aria-label="Chat options" bind:this={panel}>
      <label class="row" title="The agent gets the start and the end of output that is hundreds of lines long. The full output stays here.">
        <input type="checkbox" checked={on} onchange={(e) => toggle(e.currentTarget.checked)} aria-describedby="{uid}-d" />
        Shorten long output
      </label>
      <p id="{uid}-d" class="help">The agent gets the start and the end of command output that is hundreds of lines long. The full output stays in this chat.</p>
      <button type="button" class="btn" disabled={!usable || !!busy} onclick={() => summarize()}>New chat from summary</button>
      <button type="button" class="btn" disabled={!usable || !!busy} aria-describedby="{uid}-r{usable ? '' : ' ' + uid + '-w'}" onclick={() => summarize(true)}>Get a second opinion</button>
      {#if busy}<p class="help" role="status">Writing the summary…</p>{:else if !usable}<p class="help" id="{uid}-w">Both need a message in this chat, and the agent must not be replying.</p>{/if}
      <p id="{uid}-r" class="help">Another agent reviews this chat's summary and the uncommitted changes in its folder. You choose the agent and send the request.</p>
      {#if usage}
        <div class="meter" class:near role="meter" aria-label="Context used" aria-valuemin="0" aria-valuemax={usage.size} aria-valuenow={usage.used} aria-valuetext={`${tokens(usage.used)} of ${tokens(usage.size)} tokens`}>
          <span>Context {tokens(usage.used)} of {tokens(usage.size)}{near ? ', nearly full' : ''}</span>
          <span class="bar"><span class="fill" style:width="{percent}%"></span></span>
        </div>
        {#if usage.cost != null}<p class="help" title="Cost of this chat so far, as the agent reports it">Cost so far {costText(usage.cost, usage.currency)}</p>{/if}
      {/if}
      {#if plan}
        <p class="help" id="{uid}-u">Plan usage of {plan.name}</p>
        <ul class="plan" aria-labelledby="{uid}-u">{#each plan.all.split('\n') as w (w)}<li>{w}</li>{/each}</ul>
      {/if}
      <ChatGroupForm {id} {info} done={() => close(false)} />
      {#if error}<p class="error" role="alert">{error}</p>{/if}
    </div>
  {/if}
</span>

<style>
  .wrap {
    position: relative;
    flex: none;
  }
  .icon {
    position: relative;
    width: var(--control-h);
    padding: 0;
    color: var(--muted);
  }
  .icon:hover:not(:disabled) {
    color: var(--fg);
  }
  .dot {
    position: absolute;
    top: 5px;
    right: 5px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--attention);
  }
  .panel {
    position: absolute;
    z-index: 5;
    top: 100%;
    right: 0;
    display: grid;
    gap: var(--s-2);
    width: min(320px, calc(100vw - var(--s-4) * 2));
    padding: var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-l);
    background: var(--raised);
    box-shadow: var(--shadow-menu);
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-height: var(--control-h);
  }
  .help,
  .error {
    margin: 0;
  }
  .plan {
    margin: 0;
    padding: 0;
    color: var(--muted);
    font-size: var(--fs-s);
    list-style: none;
  }
  .meter {
    display: grid;
    gap: var(--s-1);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .meter.near {
    color: var(--attention);
    font-weight: var(--fw-semibold);
  }
  .bar {
    height: 6px;
    border: 1px solid var(--border-strong);
    border-radius: var(--r-s);
    overflow: hidden;
  }
  .fill {
    display: block;
    height: 100%;
    background: var(--accent);
  }
  .near .fill {
    background: var(--attention);
  }
</style>
