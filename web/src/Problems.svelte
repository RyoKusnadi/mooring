<script lang="ts">
  // The "Problems" dialog: what the running language servers report for the open project, errors first.
  import SshModal from './SshModal.svelte'
  import { fileActions } from './lib/fileactions'
  import { countText, countProblems, limitProblems, severityName } from './lib/problems'
  import { problems } from './lib/problems.svelte'
  import { app } from './lib/state.svelte'

  let { onClose }: { onClose: () => void } = $props()

  const MAX = 500
  const all = $derived(problems.list(app.currentId))
  const view = $derived(limitProblems(all, MAX))
  const summary = $derived(countText(countProblems(all)))

  function open(path: string, line: number) {
    fileActions.openFile(path, line)
    onClose()
  }
</script>

<SshModal title="Problems" wide {onClose}>
  {#if all.length === 0}
    <p class="muted" role="status" data-first tabindex="-1">No running language server reports problems for this project. Open a file of a supported language and choose "Allow and start" in the bar over the editor. "Language servers" in the command palette lists them.</p>
  {:else}
    <p class="muted" role="status">{summary || `${all.length} ${all.length === 1 ? 'hint' : 'hints and notes'}`}. Click a row to open the file at that line.</p>
    <ul>
      {#each view.shown as p, i (i)}
        <li>
          <button type="button" class="row" data-first={i === 0 ? '' : undefined} onclick={() => open(p.path, p.line)}>
            <span class="sev s{p.severity}">
              <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true" focusable="false">
                {#if p.severity === 2}
                  <path d="M8 2 L15 14 H1 Z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
                {:else if p.severity === 3}
                  <rect x="2" y="2" width="12" height="12" rx="2" fill="none" stroke="currentColor" stroke-width="1.5" />
                {:else if p.severity === 4}
                  <path d="M8 2 L14 8 L8 14 L2 8 Z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
                {:else}
                  <circle cx="8" cy="8" r="6.2" fill="currentColor" />
                {/if}
              </svg>{severityName(p.severity)}
            </span>
            <span class="msg">{p.message}</span>
            <span class="where">{p.path}:{p.line}:{p.col}{p.source ? ` · ${p.source}` : ''}</span>
          </button>
        </li>
      {/each}
    </ul>
    {#if view.more > 0}<p class="muted" role="status">Showing {MAX} of {all.length} problems. Fix some to see the rest.</p>{/if}
  {/if}
</SshModal>

<style>
  .muted {
    margin: 0 0 var(--s-2);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  ul {
    max-height: 55dvh;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    list-style: none;
  }
  .row {
    display: grid;
    grid-template-columns: 7.5rem minmax(0, 1fr);
    gap: 2px var(--s-2);
    width: 100%;
    min-height: var(--control-h);
    padding: var(--s-1) var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .row:hover {
    background: var(--hover);
  }
  .sev {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .s1 {
    color: var(--danger);
  }
  .s2 {
    color: var(--attention);
  }
  .s3,
  .s4 {
    color: var(--muted);
  }
  .msg {
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .where {
    grid-column: 2;
    min-width: 0;
    color: var(--muted);
    font-size: var(--fs-xs);
    overflow-wrap: anywhere;
  }
  @media (pointer: coarse), (max-width: 600px) {
    .row {
      grid-template-columns: minmax(0, 1fr);
      min-height: 44px;
    }
    .where {
      grid-column: 1;
    }
  }
</style>
