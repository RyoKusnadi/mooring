<script lang="ts">
  // Stashes of the shown place: save the changes away, then view, apply, pop or drop a stash.
  import { tick, untrack } from 'svelte'
  import { actions } from './lib/actions'
  import { gitplus } from './lib/gitplus.svelte'
  import { git } from './lib/gitstore.svelte'
  import { relativeTime, type Stash } from './lib/historyapi'

  let saving = $state(false)
  let message = $state('')
  let untracked = $state(false)
  let dropping = $state<Stash | null>(null)
  let keep: HTMLButtonElement | undefined = $state()
  let field: HTMLInputElement | undefined = $state()
  let opener: HTMLButtonElement | undefined = $state()
  const busy = $derived(git.busy !== '')
  const failure = $derived(git.failure?.source === 'stash' ? git.failure : null)
  const changed = $derived((git.status?.files.length ?? 0) > 0)

  // Read again when the place changes or git says something moved (a stash made in a terminal, say).
  $effect(() => {
    void [git.project, git.folder, git.changes]
    // A worktree being removed or already gone answers NOT_FOUND to everything.
    if (git.removing && git.removing === git.folder) return
    untrack(() => void gitplus.loadStashes())
  })

  async function open() {
    saving = true
    await tick()
    field?.focus()
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (await gitplus.stashSave(message, untracked)) {
      saving = false
      message = ''
      untracked = false
      await tick()
      opener?.focus()
    }
  }

  async function ask(s: Stash) {
    dropping = s
    await tick()
    keep?.focus()
  }

  async function drop() {
    if (!dropping) return
    const s = dropping
    const ok = await gitplus.stashDrop(s.id)
    dropping = null
    await tick()
    if (ok) opener?.focus()
  }

  const title = (s: Stash) => s.message || `Stash ${s.index}`
  const view = (s: Stash) => actions.openCommit(git.folder, s.id, `Stash: ${title(s).slice(0, 24)}`)
</script>

<div class="stash">
  <div class="bar">
    <h3>Stashes <span class="count">{gitplus.stashes.length}</span></h3>
    <button type="button" class="btn ghost small" bind:this={opener} disabled={busy || saving} aria-expanded={saving} onclick={open}>Stash changes</button>
  </div>
  {#if saving}
    <form onsubmit={save} aria-label="Stash changes">
      <div class="field">
        <label for="stash-message">Message (optional)</label>
        <input id="stash-message" class="input" bind:this={field} bind:value={message} maxlength="500" autocomplete="off" />
      </div>
      <label class="check"><input type="checkbox" bind:checked={untracked} /> Include untracked files</label>
      <p class="help">{changed ? 'Staged and unstaged changes go into the stash, and the working copy goes back to the last commit.' : 'There are no changes to stash now.'}</p>
      <div class="row">
        <button type="submit" class="btn" disabled={busy || !changed && !untracked}>{git.busy === 'git.stashSave' ? 'Stashing…' : 'Stash'}</button>
        <button type="button" class="btn ghost" onclick={() => ((saving = false), opener?.focus())}>Cancel</button>
      </div>
    </form>
  {/if}
  {#if dropping}
    <div class="confirm" role="group" aria-label="Confirm drop stash">
      <p>Drop “{title(dropping)}”? Its changes are gone for good. There is no undo.</p>
      <button type="button" class="btn danger solid small" disabled={busy} onclick={drop}>Drop stash</button>
      <button type="button" class="btn small" bind:this={keep} onclick={() => ((dropping = null), opener?.focus())}>Keep it</button>
    </div>
  {/if}
  {#if failure}<p class="error pad" role="alert">{failure.message}</p>{/if}
  {#if gitplus.stashError}<p class="error pad" role="alert">{gitplus.stashError}</p>{/if}
  {#if gitplus.stashes.length > 0}
    <ul aria-label="Stashes, newest first">
      {#each gitplus.stashes as s (s.id)}
        <li>
          <p class="name"><span>{title(s)}</span> <span class="meta">on {s.branch || 'a detached head'}, {relativeTime(s.time)}</span></p>
          <div class="acts">
            <button type="button" class="btn ghost small" aria-label={`View stash ${title(s)}`} onclick={() => view(s)}>View</button>
            <button type="button" class="btn ghost small" disabled={busy} aria-label={`Apply stash ${title(s)}`} onclick={() => gitplus.stashApply(s.id, false)}>Apply</button>
            <button type="button" class="btn ghost small" disabled={busy} aria-label={`Pop stash ${title(s)}`} onclick={() => gitplus.stashApply(s.id, true)}>Pop</button>
            <button type="button" class="btn ghost small danger" disabled={busy} aria-label={`Drop stash ${title(s)}`} onclick={() => ask(s)}>Drop</button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--s-1) var(--s-2) 0 var(--s-3);
  }
  h3 {
    margin: 0;
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .count {
    color: var(--muted);
    font-weight: var(--fw-regular);
  }
  .small {
    min-height: var(--control-h-s);
    padding: 0 var(--s-1);
    font-size: var(--fs-s);
  }
  form,
  .confirm {
    display: grid;
    gap: var(--s-2);
    margin: var(--s-1) var(--s-2);
    padding: var(--s-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-l);
    background: var(--raised);
    font-size: var(--fs-s);
  }
  .confirm {
    grid-template-columns: auto auto;
    justify-content: start;
    border-color: var(--danger);
  }
  .confirm p {
    grid-column: 1 / -1;
    margin: 0;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-height: var(--control-h);
  }
  .row {
    display: flex;
    gap: var(--s-2);
  }
  .help,
  .pad {
    margin: 0;
  }
  .pad {
    padding: var(--s-1) var(--s-3);
    overflow-wrap: anywhere;
  }
  ul {
    margin: 0;
    padding: 0 var(--s-2) var(--s-1);
    list-style: none;
  }
  li {
    padding: var(--s-1);
    border-radius: var(--r-s);
  }
  li:hover {
    background: var(--hover);
  }
  .name {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .meta {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
  }
  .danger {
    color: var(--danger);
  }
  @media (pointer: coarse) {
    .small {
      min-width: 44px;
      min-height: 44px;
    }
  }
</style>
