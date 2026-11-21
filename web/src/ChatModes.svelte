<script lang="ts">
  // Model and Mode of a chat, next to the box where you write. A mode that skips asking needs a yes first.
  import { tick } from 'svelte'
  import { app } from './lib/state.svelte'
  import { chats, type ChatInfo } from './lib/chatstore.svelte'
  import { asksFirst } from './lib/chatfmt'

  let { id, info, agent }: { id: string; info: ChatInfo; agent: string } = $props()

  const live = $derived(info.state === 'idle' || info.state === 'running')
  let pendingMode = $state('')
  let keep: HTMLButtonElement | undefined = $state()
  let modeSelect: HTMLSelectElement | undefined = $state()
  let error = $state('')

  const closeConfirm = () => {
    pendingMode = ''
    modeSelect?.focus()
  }

  async function set(name: 'chat.setModel' | 'chat.setMode', value: string) {
    error = ''
    try {
      await app.run(name, { id, value })
      if (name === 'chat.setMode') chats.feed(id).noAsk = false
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not change that setting.'
    }
  }

  function pickMode(el: HTMLSelectElement) {
    const picked = info.modes?.find((m) => m.id === el.value)
    if (!asksFirst(picked)) {
      pendingMode = el.value
      el.value = info.mode
      void tick().then(() => keep?.focus())
    } else void set('chat.setMode', el.value)
  }
</script>

{#if info.models?.length}
  <label class="pick"><span class="t">Model</span>
    <select value={info.model} disabled={!live} onchange={(e) => set('chat.setModel', e.currentTarget.value)}>
      {#each info.models as m (m.id)}<option value={m.id}>{m.name}</option>{/each}
    </select>
  </label>
{/if}
{#if info.modes?.length}
  <label class="pick"><span class="t">Mode</span>
    <select bind:this={modeSelect} value={info.mode} disabled={!live} onchange={(e) => pickMode(e.currentTarget)}>
      {#each info.modes as m (m.id)}<option value={m.id}>{m.name}</option>{/each}
    </select>
  </label>
{/if}
{#if pendingMode}
  <div class="confirm" role="alertdialog" aria-label="Change mode">
    <p>This mode may let {agent} run commands and edit files without asking you first. Switch anyway?</p>
    <button type="button" class="btn danger" onclick={() => (set('chat.setMode', pendingMode), closeConfirm())}>Switch mode</button>
    <button type="button" class="btn" bind:this={keep} onclick={closeConfirm}>Keep asking</button>
  </div>
{/if}
{#if error}<p class="error" role="alert">{error}</p>{/if}

<style>
  .pick {
    display: flex;
    flex: 0 1 auto;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    color: var(--muted);
    font-size: var(--fs-s);
    font-weight: 600;
  }
  select {
    min-height: calc(var(--control-h) - 4px);
    max-width: 16ch;
    padding: 0 var(--s-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-s);
    background: var(--raised);
    color: var(--fg);
    font: inherit;
    font-weight: 400;
  }
  select:disabled {
    background: var(--surface);
    color: var(--faint);
    border-color: var(--border);
    cursor: not-allowed;
  }
  .confirm {
    display: flex;
    flex: 1 1 100%;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
  }
  .confirm p {
    flex: 1 1 30ch;
    margin: 0;
  }
  .error {
    flex: 1 1 100%;
    margin: 0;
  }
  /* A phone: the words "Model" and "Mode" move into the select's name. */
  @media (max-width: 900px) {
    .pick {
      flex: 1 1 0;
    }
    select {
      width: 100%;
      max-width: none;
      min-height: var(--control-h);
      font-size: 16px; /* 16px stops iOS zooming into the select */
    }
    .t {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
      white-space: nowrap;
    }
  }
</style>
