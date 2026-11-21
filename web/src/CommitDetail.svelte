<script lang="ts">
  // One commit or stash: its whole message and the files it changed. A file opens its diff on its own tab.
  import { actions } from './lib/actions'
  import { app } from './lib/state.svelte'
  import { gitError } from './lib/gitapi'
  import { fullTime, getCommit, refWord, sameWho, shortId, type CommitInfo } from './lib/historyapi'

  let { folder, id }: { folder: string; id: string } = $props()

  let info = $state<CommitInfo | null>(null)
  let error = $state('')

  $effect(() => {
    const [project, wanted, place] = [app.currentId, id, folder]
    const ctl = new AbortController()
    info = null
    error = ''
    getCommit(project, place, wanted, ctl.signal).then(
      (r) => !ctl.signal.aborted && (info = r),
      (e: unknown) => !ctl.signal.aborted && (error = gitError(e, 'Could not read this commit. Try again.').message),
    )
    return () => ctl.abort()
  })

  const word = (c: string) => ({ added: 'new', modified: 'modified', deleted: 'deleted', renamed: 'renamed', typechange: 'type changed' })[c] ?? c
</script>

<article aria-label="Commit details" aria-busy={!info && !error}>
  {#if error}
    <p class="error" role="alert">{error}</p>
  {:else if !info}
    <p class="note" role="status">Reading the commit…</p>
  {:else}
    {@const c = info.commit}
    <h2>{c.subject}</h2>
    <p class="meta">
      {c.author} <span class="mail">&lt;{c.email}&gt;</span>, {fullTime(c.time)}
    </p>
    <p class="meta">
      Commit <code title={c.id}>{c.id}</code>
      {#if c.parents.length > 0}<br />{c.parents.length === 1 ? 'Parent' : 'Parents'} {#each c.parents as p, i (p)}{i ? ', ' : ''}<code title={p}>{shortId(p)}</code>{/each}{/if}
      {#if info.committer && !sameWho(info.committer, c.author, c.email)}<br />Committed by {info.committer}{/if}
    </p>
    {#if c.refs.length > 0}
      <p class="meta">On: {#each c.refs as r, i (r.kind + r.name)}{i ? ', ' : ''}<span>{r.name} ({refWord(r.kind)})</span>{/each}</p>
    {/if}
    {#if info.message.trim() !== c.subject.trim()}
      <pre class="message">{info.message.trim()}</pre>
    {/if}
    <h3>{info.files.length} {info.files.length === 1 ? 'file' : 'files'} changed{c.parents.length > 1 ? ', compared with the first parent' : ''}</h3>
    {#if info.files.length === 0}
      <p class="note">This commit changes no files.</p>
    {:else}
      <ul>
        {#each info.files as f (f.path)}
          <li>
            <button type="button" class="file" onclick={() => actions.openCommitDiff(folder, id, f.path)}>
              <span class="word">{word(f.change)}</span>
              <span class="path">{f.path}{#if f.origPath}<span class="was"> (was {f.origPath})</span>{/if}</span>
              {#if f.binary}<span class="was">binary</span>{/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
    {#if info.truncated}<p class="note">Only the first 5000 files are listed.</p>{/if}
  {/if}
</article>

<style>
  article {
    display: grid;
    align-content: start;
    gap: var(--s-2);
    min-height: 0;
    padding: var(--s-3) var(--s-4);
    overflow-y: auto;
  }
  h2,
  h3,
  p {
    margin: 0;
  }
  h2 {
    font-size: var(--fs-l);
    line-height: 1.25;
    overflow-wrap: anywhere;
  }
  h3 {
    margin-top: var(--s-2);
    font-size: var(--fs-m);
  }
  .meta,
  .note {
    color: var(--muted);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .mail {
    color: var(--muted);
  }
  .message {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-s);
    background: var(--surface);
    font: var(--fs-s) / 1.5 var(--font-mono);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .file {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: none;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .file:hover {
    background: var(--hover);
  }
  .word {
    flex: none;
    min-width: 7ch;
    padding: 0 var(--s-1);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-s);
    font-size: var(--fs-s);
    font-weight: 600;
    text-align: center;
  }
  .path {
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .was {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  @media (pointer: coarse) {
    .file {
      min-height: 44px;
    }
  }
</style>
