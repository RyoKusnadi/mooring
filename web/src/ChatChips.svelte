<script lang="ts">
  // What a message carries along: file lines and saved command outputs, as chips with no text.
  import { chipText, chipTitle, type Chip } from './lib/tokenfmt'

  let { chips, onRemove }: { chips: Chip[]; onRemove?: (index: number) => void } = $props()
</script>

{#if chips.length > 0}
  <ul class="chips" aria-label="Attached">
    {#each chips as c, i (i)}
      <li title={chipTitle(c)}>
        <span class="label">{chipText(c)}</span>
        {#if onRemove}
          <button type="button" class="x" aria-label="Remove {chipText(c)}" onclick={() => onRemove(i)}>×</button>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  .chips {
    display: flex;
    flex-wrap: wrap;
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
  .label {
    padding: 2px var(--s-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .x {
    min-width: var(--control-h);
    min-height: var(--control-h);
    margin-block: -1px;
    border: 0;
    border-left: 1px solid var(--border);
    background: transparent;
    color: var(--muted);
    cursor: pointer;
  }
  .x:hover {
    background: var(--hover);
    color: var(--fg);
  }
</style>
