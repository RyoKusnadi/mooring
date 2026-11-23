<script lang="ts">
  // The two versions of a changed picture, side by side ("Before" and "After"), stacked on a phone.
  // A version that does not exist (an added or deleted file) says so. Any other failure, and any
  // binary file that is not a picture, gets the plain note.
  import { gitImageUrl, type ImageSide } from './lib/gitimage'
  import { isImagePath } from './lib/imagetype'

  let { project, folder, path, staged, commit = '' }: { project: string; folder: string; path: string; staged: boolean; commit?: string } = $props()

  const hint = $derived(commit ? '' : staged ? 'It is staged.' : 'Stage it to include it in a commit.')
  const picture = $derived(isImagePath(path))

  type State = 'loading' | 'ok' | 'missing' | 'failed'
  let state = $state<Record<ImageSide, State>>({ old: 'loading', new: 'loading' })
  const sides: { side: ImageSide; label: string }[] = [
    { side: 'old', label: 'Before' },
    { side: 'new', label: 'After' },
  ]
  const url = (side: ImageSide) => gitImageUrl(project, folder, path, side, { staged, commit })

  // Another file, side or commit in the same view starts over. The first run must not reset: a picture
  // that loaded before this effect ran has already said so, and an unchanged address never loads again.
  let shown = ''
  $effect(() => {
    const now = url('old') + url('new')
    if (shown && now !== shown) state = { old: 'loading', new: 'loading' }
    shown = now
  })

  // The picture alone cannot say why it failed, so ask again only then: a missing version is a 404.
  async function failed(side: ImageSide) {
    try {
      const r = await fetch(url(side), { method: 'HEAD' })
      state[side] = r.status === 404 ? 'missing' : 'failed'
    } catch {
      state[side] = 'failed'
    }
  }
</script>

{#if !picture || state.old === 'failed' || state.new === 'failed'}
  <p class="note">This is a binary file, so there are no lines to show. {hint}</p>
{:else}
  <!-- svelte-ignore a11y_no_noninteractive_tabindex: a scroll area keyboard users must reach -->
  <div class="pair" tabindex="0" role="region" aria-label="Before and after">
    {#each sides as { side, label } (side)}
      <figure>
        <figcaption>{label}</figcaption>
        <div class="stage">
          {#if state[side] === 'missing'}
            <p class="none">Not in this version</p>
          {:else}
            <img src={url(side)} alt="{label}: {path}" hidden={state[side] !== 'ok'} onload={() => (state[side] = 'ok')} onerror={() => failed(side)} />
            {#if state[side] === 'loading'}<p class="none" role="status">Loading…</p>{/if}
          {/if}
        </div>
      </figure>
    {/each}
  </div>
  {#if hint}<p class="note">{hint}</p>{/if}
{/if}

<style>
  .pair {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    flex: 1;
    gap: var(--s-3);
    min-height: 0;
    padding: var(--s-3);
    overflow: auto;
  }
  @media (max-width: 700px) {
    .pair {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  figure {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    min-width: 0;
    margin: 0;
  }
  figcaption {
    color: var(--muted);
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  /* A checkerboard shows through transparent pixels. */
  .stage {
    display: flex;
    flex: 1;
    align-items: center;
    justify-content: center;
    min-height: 120px;
    padding: var(--s-2);
    border: 1px solid var(--border);
    border-radius: var(--r-l);
    background-color: var(--raised);
    background-image: conic-gradient(var(--surface) 25%, transparent 0 50%, var(--surface) 0 75%, transparent 0);
    background-size: 16px 16px;
  }
  img {
    max-width: 100%;
    max-height: 60vh;
    object-fit: contain;
  }
  .none {
    margin: 0;
    padding: var(--s-1) var(--s-2);
    border-radius: var(--r-s);
    background: var(--surface);
    color: var(--muted);
  }
  .note {
    margin: 0;
    padding: var(--s-4);
    color: var(--muted);
  }
</style>
