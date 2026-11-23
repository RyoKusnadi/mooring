<script lang="ts">
  // Messages written while the agent replies. Each goes out when the reply ends.
  import { imageCount } from './lib/attachstore.svelte'

  let { queue, onTakeBack }: { queue: { text: string; chips: { kind: string }[] }[]; onTakeBack: (index: number) => void } = $props()

  function meta(chips: { kind: string }[]): string {
    const n = imageCount(chips)
    return (n ? `${n} ${n === 1 ? 'image' : 'images'} · ` : '') + 'Sends when the reply ends'
  }
</script>

{#if queue.length}
  <ul class="queue" aria-label="Queued messages">
    {#each queue as q, i (i)}
      <li>
        <span class="qt">{q.text}</span>
        <span class="qs">{meta(q.chips)}</span>
        <button type="button" class="btn small" onclick={() => onTakeBack(i)}>Take back<span class="sr">: {q.text.slice(0, 40)}</span></button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .queue {
    display: grid;
    gap: var(--s-1);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-2);
    border: 1px dashed var(--border-strong);
    border-radius: var(--r-m);
  }
  .qt {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .qs {
    flex: none;
    color: var(--muted);
    font-size: var(--fs-s);
  }
</style>
