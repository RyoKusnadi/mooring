<script lang="ts">
  import { composing } from './lib/keys'
  // Commit message, Commit, Push, and what the last command said.
  import { stagedOf } from './lib/gitapi'
  import { amendBlocked, headPushed, offersPull, pullRemote } from './lib/gitremote.svelte'
  import { git } from './lib/gitstore.svelte'
  import { settings } from './lib/settingsstore.svelte'
  import { app } from './lib/state.svelte'
  import { suggestError } from './lib/tokenfmt'

  let { pushOnly = false }: { pushOnly?: boolean } = $props()
  const key = /Mac|iPhone|iPad/.test(navigator.platform) ? 'Cmd+Enter' : 'Ctrl+Enter'
  const status = $derived(git.status)
  const staged = $derived(status ? stagedOf(status).length : 0)
  const busy = $derived(git.busy !== '')
  const pushed = $derived(headPushed(status))
  const noCommit = $derived(!status || status.head === '' || status.detached)
  // After a push the box cannot stay in amend mode.
  $effect(() => {
    if (pushed && git.amend) git.amend = false
  })
  // An amend may keep the old message or take only the staged files.
  const canCommit = $derived(!busy && (git.amend ? !pushed && (staged > 0 || git.message.trim() !== '') : staged > 0 && git.message.trim() !== ''))
  const hasUpstream = $derived(!!status?.upstream)
  const lines = $derived((git.failure?.message ?? '').split('\n'))

  let suggesting = $state(false)
  let suggestNote = $state('')
  let lastSuggested = ''
  // A suggestion never replaces words the user typed.
  const canSuggest = $derived(staged > 0 && !busy && !suggesting && (git.message.trim() === '' || git.message === lastSuggested))

  async function suggest() {
    suggesting = true
    suggestNote = ''
    try {
      const r = await settings.cmd<{ message: string; tokens: number }>('git.suggestMessage', { projectId: git.project || app.currentId, folder: git.folder })
      git.message = lastSuggested = r.message
      suggestNote = `Suggested with ${r.tokens.toLocaleString('en-US')} tokens. Read it before you commit.`
    } catch (e) {
      suggestNote = suggestError(e)
    } finally {
      suggesting = false
    }
  }

  function keys(e: KeyboardEvent) {
    if (e.key === 'Enter' && !composing(e) && (e.metaKey || e.ctrlKey) && canCommit) {
      e.preventDefault()
      void git.commit()
    }
  }
</script>

<div class="box">
  {#if !pushOnly}<label for="commit-message">Commit message</label>
  <textarea id="commit-message" class="input" rows="2" spellcheck="true" aria-describedby="commit-help" bind:value={git.message} onkeydown={keys}></textarea>
  <p class="help" id="commit-help">
    {#if git.amend}
      Replaces the last commit with its changes plus the {staged} staged {staged === 1 ? 'file' : 'files'}. Leave the message empty to keep it.
    {:else if staged === 0}
      Stage files to include them in a commit.
    {:else if git.message.trim() === ''}
      Write a message to commit.
    {:else if status && status.stagedOutside > 0}
      Commits {staged} staged here and {status.stagedOutside} staged elsewhere in the repository.
    {:else}
      Commits the {staged} staged {staged === 1 ? 'file' : 'files'}. {key} commits.
    {/if}
  </p>{/if}
  {#if !pushOnly || !pushed}
    <label class="check" class:off={pushed}>
      <input type="checkbox" bind:checked={git.amend} disabled={busy || noCommit || pushed} aria-describedby={pushed ? 'amend-help' : undefined} /> Amend last commit
    </label>
    {#if pushed}<p class="help" id="amend-help">{amendBlocked(status)}</p>{/if}
  {/if}
  <div class="row">
    {#if settings.value.commitJobs && !pushOnly}
      <button type="button" class="btn" disabled={!canSuggest} onclick={suggest}>{suggesting ? 'Writing…' : 'Suggest message'}</button>
    {/if}
    {#if !pushOnly}<button type="button" class="btn primary" disabled={!canCommit} onclick={() => git.commit()}>{git.busy === 'git.commit' ? (git.amend ? 'Amending…' : 'Committing…') : git.amend ? 'Amend commit' : 'Commit'}</button>{/if}
    {#if !pushOnly || !hasUpstream || (status?.ahead ?? 0) > 0}<button type="button" class="btn" disabled={busy || !status || status.detached} onclick={() => git.push(!hasUpstream)}>
      {git.busy === 'git.push' ? 'Pushing…' : hasUpstream ? (status!.ahead > 0 ? `Push ${status!.ahead} ${status!.ahead === 1 ? 'commit' : 'commits'}` : 'Push') : 'Push and set upstream'}
    </button>{/if}
  </div>
  {#if suggestNote}<p class="help" role="status">{suggestNote}</p>{/if}
  {#if git.notice}<p class="ok" role="status">{git.notice}</p>{/if}
  {#if git.failure && !git.failure.source}
    <div class="fail" role="alert">
      <p class="error">{lines[0]}</p>
      {#if offersPull(git.failure)}<button type="button" class="btn" disabled={busy} onclick={() => pullRemote()}>{git.busy === 'git.pull' ? 'Pulling…' : 'Pull'}</button>{/if}
      {#if lines.length > 1}<pre>{lines.slice(1).join('\n')}</pre>{/if}
    </div>
  {/if}
</div>

<style>
  .box {
    flex: none;
    display: grid;
    gap: var(--s-1);
    padding: var(--s-2) var(--s-3) var(--s-3);
    border-top: 1px solid var(--border);
  }
  label {
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  textarea {
    min-height: 64px;
    padding: var(--s-1) var(--s-2);
    resize: vertical;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-weight: var(--fw-regular);
  }
  .check input {
    width: 16px;
    height: 16px;
    margin: 0;
    accent-color: var(--accent);
  }
  .check.off {
    color: var(--muted);
  }
  @media (pointer: coarse) {
    .check {
      min-height: 44px;
    }
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .ok {
    margin: 0;
    color: var(--success);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .fail p {
    overflow-wrap: anywhere;
  }
  pre {
    max-height: 160px;
    margin: var(--s-1) 0 0;
    padding: var(--s-2);
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--r-l);
    background: var(--raised);
    color: var(--fg);
    font: var(--fs-s) / 1.4 var(--font-mono);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
