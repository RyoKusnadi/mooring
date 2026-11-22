<script lang="ts">
  // One diff row with a "Comment on line n" button. The comment box opens under the line; saved comments show there too.
  import DiffLine from './DiffLine.svelte'
  import { diffComments } from './lib/diffcomments.svelte'
  import type { DiffRow } from './lib/diffrows'
  import { app } from './lib/state.svelte'

  type Props = Omit<NonNullable<Parameters<typeof DiffLine>[1]>, 'row'>
  let { row, path, ...rest }: { row: DiffRow; path: string } & Props = $props()

  const project = $derived(app.currentId)
  const removed = $derived(row.type === 'line' && row.kind === '-')
  const n = $derived(row.type === 'line' ? (removed ? row.old : row.now) : null)
  const can = $derived(row.type === 'line' && (row.kind === '+' || row.kind === '-' || row.kind === ' ') && n !== null)
  const mine = $derived(n === null ? undefined : diffComments.at(project, path, n, removed))
  let open = $state(false)
  let draft = $state('')
  const uid = $props.id()

  let plus: HTMLButtonElement | undefined = $state()
  const back = () => queueMicrotask(() => plus?.focus())
  // C on a focused line opens the box, so keyboard users do not tab through a button per line.
  function key(e: KeyboardEvent) {
    if (e.key.toLowerCase() !== 'c' || e.ctrlKey || e.metaKey || e.altKey || !can) return
    if (!(e.target instanceof HTMLElement) || e.target.closest('.box, .saved, button')) return
    e.preventDefault()
    e.stopPropagation()
    edit()
  }
  function edit() {
    draft = mine?.text ?? ''
    open = true
  }
  function add() {
    if (row.type !== 'line' || n === null || !draft.trim()) return
    diffComments.save(project, { path, line: n, to: n, removed, quote: row.text, text: draft.trim() })
    open = false
    back()
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="cl" class:has={!!mine || open} onkeydown={key}>
  <DiffLine {row} {...rest} />
  {#if can}
    <button type="button" class="cbtn" tabindex="-1" bind:this={plus} aria-label={`Comment on ${removed ? 'removed ' : ''}line ${n}`} title="Comment on this line (or press C)" onclick={edit}>+</button>
  {/if}
  {#if open}
    <form class="box" onsubmit={(e) => (e.preventDefault(), add())}>
      <label class="sr" for="{uid}-t">Comment for the agent on line {n}</label>
      <textarea id="{uid}-t" class="input" rows="2" placeholder="Comment for the agent" bind:value={draft} onkeydown={(e) => e.key === 'Escape' && (e.stopPropagation(), (open = false), back())} {@attach (el: HTMLTextAreaElement) => el.focus()}></textarea>
      <span class="acts">
        <button type="submit" class="btn small primary" disabled={!draft.trim()}>Add comment</button>
        <button type="button" class="btn small" onclick={() => ((open = false), back())}>Cancel</button>
      </span>
    </form>
  {:else if mine}
    <div class="saved">
      <p>{mine.text}</p>
      <span class="acts">
        <button type="button" class="btn small ghost" aria-label={`Edit comment on line ${n}`} onclick={edit}>Edit</button>
        <button type="button" class="btn small ghost" aria-label={`Remove comment on line ${n}`} onclick={() => (diffComments.remove(project, mine.id), back())}>Remove</button>
      </span>
    </div>
  {/if}
</div>

<style>
  .cl {
    position: relative;
  }
  .cbtn {
    position: absolute;
    top: 0;
    right: var(--s-2);
    width: 24px;
    height: 21px;
    padding: 0;
    border: 1px solid var(--border);
    border-radius: var(--r-s);
    background: var(--raised);
    color: var(--muted);
    font: inherit;
    line-height: 1;
    opacity: 0;
  }
  .cl:hover .cbtn,
  .cbtn:focus-visible {
    opacity: 1;
  }
  .cbtn:hover {
    color: var(--fg);
  }
  .box,
  .saved {
    display: grid;
    gap: var(--s-2);
    margin: var(--s-1) var(--s-3) var(--s-2) 64px;
    padding: var(--s-2);
    border: 1px solid var(--border);
    border-radius: var(--r-s);
    background: var(--surface);
    font-family: var(--font-ui, inherit);
    white-space: normal;
  }
  .saved p {
    margin: 0;
    font-size: var(--fs-m);
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }
  .acts {
    display: flex;
    gap: var(--s-2);
  }
  @media (pointer: coarse) {
    .cbtn {
      position: static;
      display: block;
      width: auto;
      min-width: 44px;
      min-height: 44px;
      margin: 0 var(--s-2) var(--s-1) auto;
      opacity: 1;
    }
  }
</style>
