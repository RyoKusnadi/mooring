<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import { actions, paletteKey } from './lib/actions'
  import { isMac, showKey } from './lib/keys'
  import { emptyUi, terms } from './lib/termstore.svelte'
  import type { TermView } from './lib/termview'
  import TermKeys from './TermKeys.svelte'
  import TermFind from './TermFind.svelte'
  import { attach } from './lib/attachstore.svelte'
  import { app } from './lib/state.svelte'

  let { id }: { id: string } = $props()

  const info = $derived(terms.list[id])
  const ui = $derived(terms.ui[id] ?? emptyUi)
  const gone = $derived(terms.known && !info && !terms.opening[id])
  const openError = $derived(terms.openError[id] ?? '')
  const ended = $derived(!!info?.exited || ui.wire === 'ended')
  let stage: HTMLElement | undefined = $state()
  let box: HTMLElement | undefined = $state()
  let view: TermView | null = null
  let failed = $state(false)
  let restart: HTMLButtonElement | undefined = $state()
  let sendNote = $state('')

  function sendSelection() {
    const text = terms.views.get(id)?.selection() ?? ''
    sendNote = text.trim() ? attach.sendText(text) : 'Select some text in the terminal first.'
  }

  // The chunk can arrive after the user moved on (opened a menu, typed in a box): leave
  // their focus where it is.
  function elsewhere(b: HTMLElement): boolean {
    const a = document.activeElement
    if (!a || a === document.body || b.contains(a) || a.closest('[data-terminal]')) return false
    return !!a.closest('[role="menu"], [role="dialog"], input, textarea, [contenteditable="true"]')
  }

  // The xterm chunk loads here, the first time a terminal tab is shown.
  $effect(() => {
    if (!info || !stage || !box) return
    const s = stage
    const b = box
    let cancelled = false
    import('./lib/termview')
      .then((m) => {
        if (cancelled) return
        view = m.openView(id)
        view.mount(s, b)
        if (untrack(() => ended)) view.end()
        else if (!elsewhere(b)) view.focus()
      })
      .catch(() => (failed = true))
    return () => {
      cancelled = true
      untrack(() => view)?.unmount()
      view = null
    }
  })

  // A dead shell must not keep the keyboard: hand focus to the restart button.
  $effect(() => {
    if (!ended || !restart) return
    untrack(() => view)?.end()
    restart.focus()
  })

  let hidden = $state(document.hidden)
  function visibility() {
    hidden = document.hidden
    view?.setVisible(!hidden)
  }

  // Showing a terminal that raised Attention is seeing it; a bell in the shown terminal never counts.
  $effect(() => {
    if (info?.attention && !hidden) void app.run('terminal.seen', { id }).catch(() => {})
  })
  document.addEventListener('visibilitychange', visibility)
  onDestroy(() => document.removeEventListener('visibilitychange', visibility))
</script>

<div class="term" bind:this={box}>
  {#if gone && openError}
    <div class="note" role="alert">
      <p>The terminal could not start. {openError}</p>
      <button type="button" class="btn" onclick={() => actions.closeTab(id)}>Close tab</button>
    </div>
  {:else if gone}
    <div class="note">
      <p>This terminal is gone. Terminals do not survive a restart of Mooring.</p>
      <button type="button" class="btn primary" onclick={() => actions.restartTerminal(id)}>Start a new terminal</button>
    </div>
  {:else if failed}
    <div class="note"><p>The terminal could not start. Reload the page and try again.</p></div>
  {:else}
    <div class="stage" class:boxed={!!ui.letterbox} bind:this={stage} data-terminal={id}></div>
    {#if !info}<p class="status">Starting the shell…</p>{/if}
    {#if ui.wire === 'reconnecting'}<p class="status">Reconnecting to the terminal…</p>{/if}
    {#if ui.find}<TermFind {id} />{/if}
    {#if !ended}
      <div class="strip">
        {#if ui.letterbox}
          <p role="status">Sized by another window: {ui.letterbox.cols} × {ui.letterbox.rows}. Type here to take over the size.</p>
        {:else}
          <p class="hint">{showKey('Alt+Esc')} leaves the terminal. {paletteKey} opens commands.</p>
        {/if}
        {#if sendNote}<span class="send-note" role="alert">{sendNote}</span>{/if}
        <button type="button" class="btn ghost small" aria-keyshortcuts={isMac ? 'Meta+F' : 'Control+Shift+F'} onclick={() => actions.findInTerminal(id)}>Find in output</button>
        <button type="button" class="btn ghost small" onpointerdown={(e) => e.preventDefault()} onclick={sendSelection}>Send selection to chat</button>
      </div>
    {/if}
    {#if ended}
      <div class="ended" role="status">
        <span>{info?.ssh ? 'The connection to the host ended' : 'The shell exited'}{info?.code ? ` with code ${info.code}` : ''}.</span>
        <button type="button" class="btn primary" bind:this={restart} onclick={() => actions.restartTerminal(id)}>{info?.ssh ? 'Reconnect' : 'Start a new terminal'}</button>
      </div>
    {/if}
  {/if}
  {#if info && !ended}<TermKeys {id} />{/if}
</div>

<style>
  .term {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--bg);
  }
  .stage {
    flex: 1;
    min-height: 0;
    min-width: 0;
    padding: var(--s-1) var(--s-4);
    overflow: hidden;
  }
  .stage.boxed {
    border-right: 1px dashed var(--border-strong);
    border-bottom: 1px dashed var(--border-strong);
  }
  .stage :global(.xterm-viewport) {
    background-color: var(--bg) !important;
  }
  .stage :global(.xterm) {
    height: 100%;
  }
  .stage.boxed :global(.xterm) {
    width: max-content;
    height: max-content;
  }
  .note {
    display: grid;
    justify-items: start;
    gap: var(--s-3);
    max-width: 560px;
    margin: 0 auto;
    padding: var(--s-6) var(--s-4);
  }
  .note p,
  .status {
    margin: 0;
    color: var(--muted);
  }
  .status {
    position: absolute;
    top: var(--s-2);
    right: var(--s-3);
    font-size: var(--fs-s);
  }
  .strip {
    display: flex;
    align-items: center;
    min-height: var(--s-5);
    gap: var(--s-3);
    padding: 0 var(--s-4);
    overflow: hidden;
    background: var(--surface);
    border-top: 1px solid var(--border);
  }
  .strip p {
    flex: 1;
    margin: 0;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  @media (pointer: coarse) {
    .strip .hint {
      display: none;
    }
  }
  .send-note {
    color: var(--danger);
    font-size: var(--fs-s);
  }
  .strip .small {
    flex: none;
    min-height: 1.75rem;
    padding-block: 0;
  }
  @media (pointer: coarse) {
    .strip .small {
      min-height: var(--control-h);
    }
  }
  .ended {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-3);
    background: var(--surface);
    border-top: 1px solid var(--border);
  }
</style>
