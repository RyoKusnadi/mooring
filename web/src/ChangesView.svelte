<script lang="ts">
  // Changes as a full pane view: the files on the left with the commit box under them, the diff of the
  // selected file on the right. On a phone the list and the diff are two steps.
  import { tick, untrack } from 'svelte'
  import { actions } from './lib/actions'
  import ChangesOperation from './ChangesOperation.svelte'
  import ChangesFiles, { type Pick } from './ChangesFiles.svelte'
  import ChangesHead from './ChangesHead.svelte'
  import ChangesScope from './ChangesScope.svelte'
  import ChangesTurn from './ChangesTurn.svelte'
  import CommitBox from './CommitBox.svelte'
  import DiffLazy from './DiffLazy.svelte'
  import StashSection from './StashSection.svelte'
  import { stagedOf, unstagedOf } from './lib/gitapi'
  import { git } from './lib/gitstore.svelte'
  import { app } from './lib/state.svelte'

  const status = $derived(git.status)
  const staged = $derived(status ? stagedOf(status) : [])
  const unstaged = $derived(status ? unstagedOf(status) : [])
  let scope = $state<'now' | 'turn'>('now')
  let turnChat = $state('')
  let sel = $state<Pick | null>(null)
  let step = $state<'list' | 'diff'>('list')
  let root: HTMLElement | undefined = $state()
  let seen = git.reveal

  // The selection, or the same file on its other side, or (on a wide screen) the first file.
  const shown = $derived.by<Pick | null>(() => {
    const has = (p: Pick) => (p.staged ? staged : unstaged).some((c) => c.path === p.path)
    if (sel && has(sel)) return sel
    if (sel && has({ path: sel.path, staged: !sel.staged })) return { path: sel.path, staged: !sel.staged }
    if (app.narrow) return null
    return staged[0] ? { path: staged[0].path, staged: true } : unstaged[0] ? { path: unstaged[0].path, staged: false } : null
  })
  const stepNow = $derived(app.narrow && shown ? step : 'list')
  const clean = $derived(!!status?.repo && status.files.length === 0)
  // With nothing to commit only Push can matter: when commits are waiting, or there is no upstream yet.
  const canPush = $derived(!!status && !status.detached && !!status.head && (status.ahead > 0 || !status.upstream))
  const showCommit = $derived(!clean || git.message !== '' || canPush || !!git.notice || (!!git.failure && !git.failure.source))

  // The diff follows the selection after a short pause, so holding an arrow key does not read every file on the way.
  let diffPick = $state<Pick | null>(null)
  $effect(() => {
    const next = shown
    const quick = !diffPick || !next
    if (quick) return untrack(() => void (diffPick = next))
    const t = setTimeout(() => (diffPick = next), 120)
    return () => clearTimeout(t)
  })

  // On a phone the commit box is one line until it is needed.
  let commitOpen = $state(false)
  const commitShown = $derived(!app.narrow || commitOpen || clean || git.message !== '' || !!git.notice || (!!git.failure && !git.failure.source))
  async function openCommit() {
    commitOpen = true
    await tick()
    root?.querySelector<HTMLElement>('#commit-message')?.focus()
  }

  // Alt+G, the palette and a chat's button ask to see the changes: focus goes to the file list.
  $effect(() => {
    const r = git.reveal
    if (r === seen) return
    seen = r
    void tick().then(() => setTimeout(focusList, 0))
  })

  const focusList = () => (root?.querySelector<HTMLElement>('.files button[tabindex="0"]') ?? root?.querySelector<HTMLElement>('[data-changes-focus]') ?? root)?.focus()

  function back() {
    step = 'list'
    void tick().then(focusList)
  }

  // Read every few seconds while this view is on screen; file watches miss edits below the top folder.
  $effect(() => git.hold())
  $effect(() => {
    void app.currentId
    void git.folder
    untrack(() => ((sel = null), (diffPick = null), (commitOpen = false)))
  })
</script>

<svelte:window onfocus={() => git.refresh()} />

{#if app.ready && app.current}
  <section aria-label={`Changes in ${app.current.name}`} tabindex="-1" bind:this={root}>
    <ChangesHead />
    <ChangesScope bind:scope bind:chat={turnChat} />
    {#if scope === 'turn'}
      <ChangesTurn chat={turnChat} />
    {:else if !status}
      {#if !git.readError}<p class="hint" role="status">Reading changes…</p>{/if}
    {:else if !status.repo}
      <div class="empty">
        <p>This folder is not a git repository. To start one, open a terminal here and run <code>git init</code>.</p>
        <button type="button" class="btn small" onclick={() => actions.newTerminal()}>Open a terminal here</button>
      </div>
    {:else}
      <div class="split" class:clean data-step={stepNow}>
        <div class="left">
          <div class="scroll">
            {#if status.operation}<ChangesOperation op={status.operation} left={unstaged.filter((c) => c.kind === 'conflicted').length} />{:else if status.conflicted}<p class="warn" role="status">Some files have merge conflicts. Resolve and stage them before committing.</p>{/if}
            {#if clean}<p class="empty" role="status">No changes. Everything is committed{status.head ? ` (last commit ${status.head})` : ''}. Edited, new and deleted files show up here.</p>{/if}
            <ChangesFiles {staged} {unstaged} sel={shown} onselect={(p) => (sel = p)} onopen={() => (step = 'diff')} />
            {#if status.truncated}<p class="hint">The list is cut short because there are too many changes. Commit or discard some, then refresh.</p>{/if}
            <StashSection />
          </div>
          {#if showCommit}
            <div class="commit">
              {#if commitShown}<CommitBox pushOnly={clean && git.message === '' && !git.amend} />{:else}<button type="button" class="peek" onclick={openCommit}>Commit message{staged.length > 0 ? `, ${staged.length} staged` : ''}</button>{/if}
            </div>
          {/if}
        </div>
        <div class="right">
          {#if app.narrow}<div class="backbar"><button type="button" class="btn small" onclick={back}>Back to files</button></div>{/if}
          {#if diffPick}
            {#key `${diffPick.staged}:${diffPick.path}`}
              <DiffLazy id="changes" loading={diffPick !== shown} folder={git.folder} path={diffPick.path} staged={diffPick.staged} onflip={(s) => shown?.path === diffPick?.path && (sel = { path: diffPick!.path, staged: s })} />
            {/key}
          {:else if !clean}
            <p class="hint">Pick a file to see its changes.</p>
          {/if}
        </div>
      </div>
    {/if}
  </section>
{/if}

<style>
  section {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  section:focus {
    outline: none;
  }
  .split {
    display: grid;
    flex: 1;
    grid-template-columns: clamp(320px, 36%, 560px) minmax(0, 1fr);
    min-height: 0;
  }
  .left {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    border-right: 1px solid var(--border);
  }
  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .commit {
    flex: none;
    max-height: 50%;
    overflow-y: auto;
    border-top: 1px solid var(--border);
  }
  .right {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
  }
  .hint,
  .warn {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .hint {
    color: var(--muted);
  }
  .warn {
    color: var(--attention);
    background: var(--attention-bg);
  }
  .empty {
    max-width: 560px;
    margin: var(--s-6, 32px) auto var(--s-3);
    padding: 0 var(--s-3);
    color: var(--muted);
    font-size: var(--fs-s);
    text-align: center;
    overflow-wrap: anywhere;
  }
  div.empty {
    display: grid;
    justify-items: center;
    gap: var(--s-3);
  }
  div.empty p {
    margin: 0;
  }
  .split.clean {
    grid-template-columns: minmax(0, 1fr);
  }
  .clean .right {
    display: none;
  }
  .clean .left {
    border-right: 0;
  }
  .clean .commit,
  .clean :global(.stashes) {
    width: 100%;
    max-width: 560px;
    margin-inline: auto;
  }
  .peek {
    display: block;
    width: 100%;
    min-height: 44px;
    padding: 0 var(--s-3);
    border: 0;
    background: var(--surface);
    color: var(--muted);
    font: inherit;
    font-size: var(--fs-s);
    text-align: left;
  }
  .peek:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .backbar {
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .small {
    min-height: var(--control-h);
    font-size: var(--fs-s);
  }
  @media (max-width: 900px) {
    .split {
      grid-template-columns: minmax(0, 1fr);
    }
    .split[data-step='list'] .right,
    .split[data-step='diff'] .left {
      display: none;
    }
    .small {
      min-height: 44px;
    }
  }
</style>
