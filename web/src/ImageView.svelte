<script lang="ts">
  // A picture in the file tab: centred and fit to the pane, or at its own size. It loads from the
  // raw route, which only serves images; an SVG shown through <img> cannot run script.
  import { formatBytes, baseName } from './lib/paths'
  import { rawUrl } from './lib/imagetype'

  let { projectId, path, size }: { projectId: string; path: string; size: number } = $props()

  let fit = $state(true)
  let natural = $state<{ w: number; h: number } | null>(null)
  let failed = $state(false)
</script>

<div class="image">
  <div class="bar">
    <span class="info" role="status">{#if natural}{natural.w} × {natural.h} px, {formatBytes(size)}{:else if !failed}Loading the image…{/if}</span>
    {#if !failed}
      <div class="switch" role="group" aria-label="Image size">
        <button type="button" aria-pressed={fit} onclick={() => (fit = true)}>Fit</button>
        <button type="button" aria-pressed={!fit} onclick={() => (fit = false)}>100%</button>
      </div>
    {/if}
  </div>
  {#if failed}
    <p class="error" role="alert">{baseName(path)} could not be shown. It may be damaged, or not the kind of image its name says.</p>
  {/if}
  <div class="stage" class:fit class:failed>
    <img
      src={rawUrl(projectId, path, size)}
      alt={baseName(path)}
      hidden={failed}
      onload={(e) => {
        const el = e.currentTarget as HTMLImageElement
        natural = { w: el.naturalWidth, h: el.naturalHeight }
      }}
      onerror={() => (failed = true)}
    />
  </div>
</div>

<style>
  .image {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }
  .bar {
    display: flex;
    flex: none;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-3);
    border-bottom: 1px solid var(--border);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .switch {
    display: flex;
    gap: var(--s-1);
  }
  .switch button {
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 1px solid transparent;
    border-radius: var(--r-s);
    background: none;
    color: var(--muted);
    font: inherit;
  }
  .switch button:hover {
    background: var(--hover);
  }
  .switch button[aria-pressed='true'] {
    background: var(--selected);
    color: var(--fg);
  }
  /* A checkerboard shows through transparent pixels. */
  .stage {
    display: flex;
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-3);
    background-color: var(--raised);
    background-image: conic-gradient(var(--surface) 25%, transparent 0 50%, var(--surface) 0 75%, transparent 0);
    background-size: 16px 16px;
  }
  .stage.failed {
    display: none;
  }
  @media (max-width: 900px), (pointer: coarse) {
    .switch button {
      min-width: 44px;
      min-height: 44px;
    }
  }
  img {
    flex: none;
    margin: auto;
  }
  .fit img {
    flex: 0 1 auto;
    min-width: 0;
    min-height: 0;
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
  }
  .error {
    margin: 0;
    padding: var(--s-4);
    color: var(--danger);
  }
</style>
