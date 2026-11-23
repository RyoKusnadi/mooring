<script lang="ts">
  // One popover for branches: type to filter, pick one to switch, or make the typed name a new branch
  // at the current commit. Esc, a click outside, or a finished switch closes it.
  import { onMount } from 'svelte'
  import { branchFailureText, canCreate, filterBranches } from './lib/branchfilter'
  import { gitplus } from './lib/gitplus.svelte'
  import { git } from './lib/gitstore.svelte'
  import { relativeTime } from './lib/historyapi'

  let { onclose }: { onclose: () => void } = $props()

  let name = $state('')
  let hi = $state(0)
  let field: HTMLInputElement | undefined = $state()
  let root: HTMLElement | undefined = $state()
  const busy = $derived(git.busy !== '')
  const names = $derived(gitplus.branches.map((b) => b.name))
  const shown = $derived(filterBranches(names, name))
  const make = $derived(canCreate(names, name))
  // Options in order: matching branches, then "make this branch".
  const count = $derived(shown.length + (make ? 1 : 0))
  const at = $derived(Math.min(hi, Math.max(count - 1, 0)))
  const failure = $derived(git.failure?.source === 'branch' ? branchFailureText(git.failure) : '')

  onMount(() => {
    void gitplus.loadBranches()
    field?.focus()
    const outside = (e: PointerEvent) => {
      if (!root?.parentElement?.contains(e.target as Node)) onclose()
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  })

  async function pick(branch: string) {
    if (await gitplus.switchTo(branch)) onclose()
    else field?.focus()
  }

  async function create() {
    if (await gitplus.createBranch(name.trim())) onclose()
    else field?.focus()
  }

  const act = (i: number) => (i < shown.length ? pick(shown[i]) : create())

  function keys(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation()
      onclose()
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (count > 0) hi = (at + (e.key === 'ArrowDown' ? 1 : count - 1)) % count
    } else if (e.key === 'Enter' && e.target === field) {
      e.preventDefault()
      if (count > 0 && !busy && !(shown[at] && shown[at] === current)) void act(at)
    }
  }

  const current = $derived(gitplus.branches.find((b) => b.head)?.name)
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions: Escape closes the popover from any control in it -->
<div class="menu" bind:this={root} role="group" aria-label="Branches" onkeydown={keys}>
  <label for="branch-filter">Switch to a branch, or name a new one</label>
  <input id="branch-filter" class="input mono" bind:this={field} bind:value={name} oninput={() => (hi = 0)} maxlength="200" autocomplete="off" spellcheck="false" aria-controls="branch-options" />
  {#if failure}<p class="error" role="alert">{failure}</p>{/if}
  {#if gitplus.branchError}
    <p class="error" role="alert">{gitplus.branchError}</p>
  {:else if gitplus.branches.length === 0}
    <p class="help" role="status">Reading the branches…</p>
  {:else}
    <ul id="branch-options" aria-label="Local branches, newest commit first">
      {#each shown as n, i (n)}
        {@const b = gitplus.branches.find((x) => x.name === n)!}
        <li>
          <button type="button" class="pick" class:on={i === at} disabled={busy || b.head} aria-current={b.head ? 'true' : undefined} aria-label={b.head ? `${n}, current branch` : `Switch to ${n}`} onclick={() => pick(n)}>
            <code>{n}</code>
            <span class="meta">{b.head ? 'current' : b.time ? relativeTime(b.time) : ''}{b.upstream ? `, tracks ${b.upstream}` : ''}</span>
          </button>
        </li>
      {/each}
      {#if make}
        <li>
          <button type="button" class="pick make" class:on={at === shown.length} disabled={busy} onclick={create}>
            {git.busy === 'git.branchCreate' ? 'Making…' : 'Make branch'} <code>{name.trim()}</code> <span class="meta">from the current commit</span>
          </button>
        </li>
      {/if}
    </ul>
    {#if count === 0}<p class="help">No branches yet. Type a name to make the first one.</p>{/if}
  {/if}
</div>

<style>
  .menu {
    position: absolute;
    z-index: 20;
    top: 100%;
    left: 0;
    display: grid;
    gap: var(--s-2);
    width: min(420px, 92vw);
    padding: var(--s-2);
    font-size: var(--fs-s);
  }
  label {
    font-weight: var(--fw-semibold);
  }
  ul {
    max-height: 260px;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    list-style: none;
  }
  .pick {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0 var(--s-2);
    width: 100%;
    min-height: var(--row-h);
    padding: 2px var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: none;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .pick:hover:not(:disabled) {
    background: var(--hover);
  }
  .pick.on:not(:disabled) {
    background: var(--selected);
  }
  .pick:disabled {
    cursor: default;
  }
  .pick:disabled code {
    font-weight: var(--fw-semibold);
  }
  code {
    overflow-wrap: anywhere;
  }
  .meta {
    color: var(--muted);
  }
  p {
    margin: 0;
  }
</style>
