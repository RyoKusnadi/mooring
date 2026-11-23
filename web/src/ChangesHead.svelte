<script lang="ts">
  // The top of the Changes view: which place (project or worktree), the branch popover, and what the last read said.
  import { tick, untrack } from 'svelte'
  import BranchMenu from './BranchMenu.svelte'
  import { actions } from './lib/actions'
  import { branchFailureText } from './lib/branchfilter'
  import { gitplus } from './lib/gitplus.svelte'
  import { fetchRemote, pullLabel, pullRemote } from './lib/gitremote.svelte'
  import { git } from './lib/gitstore.svelte'
  import { baseName } from './lib/paths'

  const status = $derived(git.status)
  const known = $derived(git.worktrees.some((w) => w.folder === git.folder))
  const showPlace = $derived(git.worktrees.length > 0 || git.folder !== '')
  let removing = $state(false)
  let branches = $state(false)
  let branchBtn: HTMLButtonElement | undefined = $state()

  // Switch branch from the palette opens the popover once, even if this view was not open yet.
  $effect(() => {
    if (git.branchRequest === git.branchHandled) return
    git.branchHandled = git.branchRequest
    untrack(() => (branches = true))
  })

  async function remove() {
    const folder = git.folder
    if (!(await git.removeWorktree())) return
    removing = false
    actions.closePlace(folder)
  }

  function closeMenu() {
    branches = false
    void tick().then(() => branchBtn?.focus())
  }
</script>

<div class="head">
  <h2 class="sr">Changes</h2>
  {#if showPlace}
    <div class="field">
      <label for="place">Worktree</label>
      <span class="select"><select id="place" class="input" value={git.folder} onchange={(e) => ((removing = false), git.setFolder(e.currentTarget.value))}>
        <option value="">Project folder</option>
        {#each git.worktrees as w (w.folder)}<option value={w.folder}>Worktree {w.branch}</option>{/each}
        {#if git.folder && !known}<option value={git.folder}>Worktree {baseName(git.folder)}</option>{/if}
      </select></span>
    </div>
  {/if}
  {#if status?.repo}
    <div class="branch" role="group" aria-label="Branch">
      <button type="button" class="bname" data-changes-focus bind:this={branchBtn} aria-expanded={branches} aria-haspopup="true" onclick={() => (branches = !branches)}>
        <code>{status.detached ? `detached at ${status.head}` : status.branch || 'no commits yet'}</code>
        <span class="change">Switch branch</span>
      </button>
      <span class="sync">
        {#if status.upstream}{status.ahead} ahead, {status.behind} behind <span class="up">{status.upstream}</span>{:else if !status.detached}No upstream{/if}
      </span>
      {#if branches}<BranchMenu onclose={closeMenu} />{/if}
    </div>
  {/if}
  <span class="grow"></span>
  <button type="button" class="btn small" disabled={git.loading} onclick={() => (git.refresh(), void git.loadWorktrees(), void gitplus.loadStashes())}>{git.loading ? 'Reading…' : 'Refresh'}</button>
  {#if status?.repo}<button type="button" class="btn small" onclick={() => actions.openHistory(git.folder)}>History</button>
    <button type="button" class="btn small" disabled={git.busy !== ''} onclick={() => fetchRemote()}>{git.busy === 'git.fetch' ? 'Fetching…' : 'Fetch'}</button>
    {#if pullLabel(status)}<button type="button" class="btn small primary" disabled={git.busy !== ''} onclick={() => pullRemote()}>{git.busy === 'git.pull' ? 'Pulling…' : pullLabel(status)}</button>{/if}
  {/if}
  {#if git.folder}
    <button type="button" class="btn ghost small" onclick={() => actions.terminalIn(git.folder, git.worktree?.branch ?? baseName(git.folder))}>Terminal here</button>
    <button type="button" class="btn ghost small danger" disabled={git.busy !== ''} onclick={() => (removing = true)}>Remove worktree</button>
  {/if}
</div>
{#if removing}
  <div class="confirm" role="group" aria-label="Confirm remove worktree">
    <p>Remove this worktree? The folder goes to the trash. Its branch and commits stay. Mooring checks where terminals were opened, so close any terminal you moved into this folder.</p>
    {#if git.worktree?.unpushed}<p class="warn">Branch {git.worktree.branch} keeps its {git.worktree.unpushed} {git.worktree.unpushed === 1 ? 'commit' : 'commits'} that {git.worktree.unpushed === 1 ? 'is' : 'are'} not on a remote.</p>{/if}
    {#if git.failure?.source === 'worktree'}<p class="error" role="alert">{git.failure.message}</p>{/if}
    <button type="button" class="btn danger solid small" disabled={git.busy !== ''} onclick={remove}>Remove folder</button>
    <button type="button" class="btn small" onclick={() => (removing = false)}>Keep it</button>
  </div>
{/if}
{#if !branches && git.failure?.source === 'branch'}<p class="error" role="alert">{branchFailureText(git.failure)}</p>{/if}
{#if git.readError}
  <p class="error" role="alert">{git.readError}{#if git.readRetry} <button type="button" class="link" onclick={() => git.refresh()}>Try again</button>{/if}</p>
{/if}

<style>
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .grow {
    flex: 1;
  }
  .field {
    min-width: 160px;
  }
  label {
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  select {
    text-overflow: ellipsis;
  }
  .small {
    flex: none;
    min-height: var(--control-h-s);
    padding: 0 var(--s-2);
    font-size: var(--fs-s);
  }
  .danger {
    color: var(--danger);
  }
  .branch {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  .bname {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-m);
    background: var(--raised);
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .bname:hover {
    background: var(--hover);
  }
  .bname code {
    overflow-wrap: anywhere;
    font-weight: var(--fw-semibold);
  }
  .change {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .sync {
    color: var(--muted);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .up {
    font-family: var(--font-mono);
  }
  .error {
    margin: 0;
    padding: var(--s-1) var(--s-3);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .confirm {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-2);
    margin: var(--s-1) var(--s-3);
    padding: var(--s-2);
    border: 1px solid var(--danger);
    border-radius: var(--r-l);
    background: var(--raised);
    font-size: var(--fs-s);
  }
  .confirm p {
    flex: 1 1 100%;
    margin: 0;
  }
  .warn {
    color: var(--fg);
    font-weight: var(--fw-semibold);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
  }
  @media (pointer: coarse) {
    .small {
      min-height: 44px;
    }
  }
  /* On a phone the header is as short as it can be, so the file list gets the screen. */
  @media (max-width: 900px) {
    .head {
      align-items: center;
      gap: var(--s-1);
      padding: var(--s-1) var(--s-2);
    }
    .field {
      min-width: 0;
    }
    .field label {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
    }
    .change,
    .grow {
      display: none;
    }
    .small {
      padding: 0 var(--s-1);
    }
  }
</style>
