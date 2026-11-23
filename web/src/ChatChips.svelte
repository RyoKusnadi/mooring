<script lang="ts">
  // What a message carries along: file lines and saved command outputs as text chips, and images as
  // chips with a thumbnail, their name and size. Images show only when `chatId` is given.
  import { fmtBytes, imageUrl } from './lib/images'
  import { chipText, chipTitle, type Chip } from './lib/tokenfmt'

  let { chips, chatId = '', onRemove }: { chips: Chip[]; chatId?: string; onRemove?: (index: number) => void } = $props()
  const shown = $derived(chips.map((c, i) => ({ c, i })).filter(({ c }) => c.kind !== 'image' || chatId))
</script>

{#if shown.length > 0}
  <ul class="chips" aria-label="Attached">
    {#each shown as { c, i } (i)}
      <li class:img={c.kind === 'image'} title={chipTitle(c)}>
        {#if c.kind === 'image' && c.imageId}
          <img class="thumb" src={imageUrl(chatId, c.imageId)} width="40" height="40" alt="" />
          <span class="meta"><span class="nm">{c.name}</span><span class="sz">· {fmtBytes(c.size ?? 0)}</span></span>
        {:else}
          <span class="label">{chipText(c)}</span>
        {/if}
        {#if onRemove}
          <button type="button" class="x" aria-label="Remove {chipText(c)}" onclick={() => onRemove(i)}>
            <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="M4.5 4.5l7 7M11.5 4.5l-7 7" /></svg>
          </button>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  .chips {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: inline-flex;
    align-items: center;
    max-width: 100%;
    border: 1px solid var(--border-strong);
    border-radius: var(--r-s);
    background: var(--raised);
    font-size: var(--fs-s);
  }
  li.img {
    gap: var(--s-2);
    width: 264px;
    padding: 3px 0 3px 3px;
    border-radius: var(--r-m);
  }
  .label {
    padding: 2px var(--s-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .thumb {
    display: block;
    flex: none;
    width: 40px;
    height: 40px;
    border-radius: var(--r-s);
    background: var(--hover);
    object-fit: cover;
  }
  .meta {
    display: flex;
    flex: 1;
    align-items: baseline;
    gap: var(--s-1);
    min-width: 0;
  }
  .nm {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sz {
    flex: none;
    color: var(--muted);
  }
  .x {
    display: inline-grid;
    flex: none;
    place-items: center;
    min-width: var(--control-h-s);
    min-height: var(--control-h-s);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--muted);
    cursor: pointer;
  }
  li.img .x {
    margin-right: 3px;
  }
  .x:hover {
    background: var(--hover);
    color: var(--fg);
  }
  @container (max-width: 520px) {
    li.img {
      width: 100%;
    }
  }
</style>
