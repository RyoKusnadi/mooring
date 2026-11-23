<script lang="ts">
  // Grey blocks that stand in for a list while it loads. Used by the Files, Changes and Board lists.
  // label is what screen readers hear, for example "Loading files".
  let { rows = 3, label = 'Loading' }: { rows?: number; label?: string } = $props()
</script>

<div class="skeleton" aria-hidden="true">
  {#each { length: rows } as _, i (i)}
    <div class="skel" style:width={i === rows - 1 ? '60%' : '100%'}></div>
  {/each}
</div>
<span class="sr">{label}</span>

<style>
  .skeleton {
    display: grid;
    gap: var(--s-3);
  }
  .skel {
    height: var(--s-3);
    border-radius: var(--r-s);
    background: var(--hover);
    animation: pulse 1.2s var(--curve) infinite alternate;
  }
  @keyframes pulse {
    from {
      opacity: 0.5;
    }
    to {
      opacity: 1;
    }
  }
</style>
