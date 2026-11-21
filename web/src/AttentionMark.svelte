<script lang="ts">
  // The amber dot for something waiting on the user. The words are for screen readers and the
  // hover title: a row shows the dot only, so names keep their room.
  let { label = 'Needs permission' }: { label?: string } = $props()
</script>

<span class="dot" role="img" aria-label={label} title={label}></span>

<style>
  .dot {
    display: inline-block;
    flex: none;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--attention);
  }
</style>
