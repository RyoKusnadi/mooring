<script lang="ts">
  // A PDF in the file tab, drawn by the browser's own viewer in a frame. The raw route only serves a
  // file that starts like a PDF (415 otherwise) and nothing over 50 MB (413); a frame cannot report
  // either, so a HEAD request asks first. The frame has no sandbox attribute: Chrome refuses a PDF in one.
  import { rawUrl } from './lib/imagetype'
  import { baseName } from './lib/paths'

  let { projectId, path, size, onText }: { projectId: string; path: string; size: number; onText?: () => void } = $props()

  let state = $state<'checking' | 'ok' | 'notpdf' | 'large' | 'failed'>('checking')
  const url = $derived(rawUrl(projectId, path, size))

  $effect(() => {
    const u = url
    state = 'checking'
    const stop = new AbortController()
    fetch(u, { method: 'HEAD', signal: stop.signal })
      .then((r) => (state = r.ok ? 'ok' : r.status === 415 ? 'notpdf' : r.status === 413 ? 'large' : 'failed'))
      .catch(() => !stop.signal.aborted && (state = 'failed'))
    return () => stop.abort()
  })
</script>

<div class="pdf">
  {#if state === 'ok'}
    <div class="bar"><a href={url} target="_blank" rel="noopener">Open in a new tab</a></div>
    <iframe src={url} title={`${baseName(path)} (PDF)`}></iframe>
  {:else if state === 'checking'}
    <p class="note" role="status">Opening {baseName(path)}…</p>
  {:else}
    <p class="note" role="alert">
      {#if state === 'notpdf'}{baseName(path)} is not a PDF, whatever its name says.
      {:else if state === 'large'}{baseName(path)} is over 50 MB, which is more than Mooring shows. Open it with another program.
      {:else}{baseName(path)} could not be shown. Try again later.{/if}
    </p>
    {#if state === 'notpdf' && onText}<div class="text"><button type="button" class="btn" onclick={onText}>Open as text</button></div>{/if}
  {/if}
</div>

<style>
  .pdf {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }
  .bar {
    flex: none;
    padding: var(--s-1) var(--s-3);
    border-bottom: 1px solid var(--border);
    font-size: var(--fs-s);
  }
  a {
    display: inline-flex;
    align-items: center;
    min-height: var(--control-h);
  }
  iframe {
    flex: 1;
    min-height: 0;
    width: 100%;
    border: 0;
    background: var(--raised);
  }
  .note {
    margin: 0;
    padding: var(--s-4);
  }
  .text {
    padding: 0 var(--s-4);
  }
  @media (pointer: coarse) {
    a {
      min-height: 44px;
    }
  }
</style>
