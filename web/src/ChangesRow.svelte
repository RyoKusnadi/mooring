<script lang="ts">
  // One file in the Changes list: letter, the name in full, the dimmed folder, then Stage and Discard.
  import Icon from './Icon.svelte'
  import { sideLetter, sideWord, type Change } from './lib/gitapi'
  import { baseName } from './lib/paths'

  let { c, staged, picked, key, tab, busy, detail, onfocus, onopen, onkeys, ontoggle, ondiscard }: {
    c: Change
    staged: boolean
    picked: boolean
    key: string
    tab: boolean
    busy: boolean
    detail: string
    onfocus: () => void
    onopen: () => void
    onkeys: (e: KeyboardEvent) => void
    ontoggle: () => void
    ondiscard: () => void
  } = $props()
</script>

<li class:picked data-row={key}>
  <button
    type="button"
    class="open"
    data-key={key}
    tabindex={tab ? 0 : -1}
    title={c.path}
    aria-label={`${c.path}, ${sideWord(c, staged)}. Show changes`}
    aria-current={picked ? 'true' : undefined}
    {onfocus}
    onclick={onopen}
    onkeydown={onkeys}
    onkeyup={(e) => e.key === ' ' && e.preventDefault()}
  >
    <span class="badge {sideLetter(c, staged)}" aria-hidden="true">{sideLetter(c, staged)}</span>
    <span class="text">
      <span class="name">{baseName(c.path)}</span>
      {#if detail}<span class="dir">{detail}</span>{/if}
    </span>
  </button>
  <button type="button" class="icon-btn" tabindex="-1" disabled={busy} aria-label={`${staged ? 'Unstage' : 'Stage'} ${c.path}`} title={staged ? 'Unstage (Space)' : 'Stage (Space)'} onclick={ontoggle}><Icon name={staged ? 'minus' : 'plus'} /></button>
  {#if !staged && c.kind !== 'conflicted'}
    <button type="button" class="icon-btn danger" tabindex="-1" disabled={busy} aria-label={`Discard changes to ${c.path}`} title="Discard (Delete)" onclick={ondiscard}><Icon name="trash" /></button>
  {/if}
</li>

<style>
  li {
    display: flex;
    align-items: center;
    border-radius: var(--r-s);
    box-shadow: inset 2px 0 0 transparent;
  }
  li:hover {
    background: var(--hover);
  }
  li.picked {
    background: var(--selected);
    box-shadow: inset 2px 0 0 var(--accent);
  }
  .open {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
    min-height: var(--control-h);
    padding: var(--s-1);
    border: 0;
    background: none;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  /* The name stays whole (it wraps before it would run under the icons); the folder gives way first. */
  .text {
    display: flex;
    flex: 1;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0 var(--s-1);
    min-width: 0;
  }
  .name {
    max-width: 100%;
    overflow-wrap: anywhere;
  }
  .badge {
    flex: none;
    width: 18px;
    height: 18px;
    border-radius: var(--r-s);
    background: var(--hover);
    color: var(--muted);
    font-size: 11px;
    font-weight: 600;
    line-height: 18px;
    text-align: center;
  }
  .badge.M,
  .badge.T,
  .badge.R {
    background: var(--hover);
    color: var(--fg);
  }
  .badge.A {
    background: var(--diff-add);
    color: var(--success);
  }
  .badge.D,
  .badge.U {
    background: var(--diff-del);
    color: var(--danger);
  }
  .dir {
    flex: 1 1 8ch;
    min-width: 0;
    color: var(--muted);
    font-size: var(--fs-s);
    white-space: normal;
    overflow-wrap: anywhere;
  }
  .danger {
    color: var(--danger);
  }
</style>
