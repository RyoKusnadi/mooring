<script lang="ts">
  // The images of a sent message: 96px thumbnails with their names. One opens in a viewer.
  import ImageViewer from './ImageViewer.svelte'
  import { fmtBytes, imageUrl } from './lib/images'
  import type { Chip } from './lib/tokenfmt'

  let { chatId, images }: { chatId: string; images: Chip[] } = $props()
  let open = $state<Chip | null>(null)
  let from: HTMLButtonElement | undefined
  let missing = $state<Record<string, boolean>>({})
</script>

<ul class="sent" aria-label="Attached images">
  {#each images as im (im.imageId)}
    <li>
      <button type="button" aria-label="View {im.name}" onclick={(e) => ((from = e.currentTarget), (open = im))}>
        {#if missing[im.imageId ?? '']}
          <span class="thumb gone">Image not found</span>
        {:else}
          <img class="thumb" src={imageUrl(chatId, im.imageId ?? '')} width="96" height="96" alt="" loading="lazy" onerror={() => (missing[im.imageId ?? ''] = true)} />
        {/if}
        <span class="fn">{im.name}</span>
      </button>
    </li>
  {/each}
</ul>
{#if open}
  <ImageViewer src={imageUrl(chatId, open.imageId ?? '')} name={open.name ?? 'Image'} size={fmtBytes(open.size ?? 0)} onClose={() => ((open = null), from?.focus())} />
{/if}

<style>
  .sent {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-3);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  button {
    display: grid;
    gap: var(--s-1);
    width: 96px;
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    text-align: left;
  }
  .thumb {
    display: block;
    width: 96px;
    height: 96px;
    border-radius: var(--r-m);
    background: var(--hover);
    object-fit: cover;
    outline: 1px solid var(--border-strong);
    outline-offset: -1px;
    transition: outline-color var(--ease);
  }
  button:hover .thumb {
    outline-color: var(--fg);
  }
  .gone {
    display: grid;
    place-items: center;
    padding: var(--s-1);
    color: var(--muted);
    font-size: var(--fs-s);
    line-height: var(--lh-s);
    text-align: center;
  }
  .fn {
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-s);
    line-height: var(--lh-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
