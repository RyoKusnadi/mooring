<script lang="ts">
  // "Loading projects…", and after 5 seconds without an answer, what is wrong and a way to try again.
  import { onMount } from 'svelte'
  import { app } from './lib/state.svelte'

  const SLOW_S = 5
  let seconds = $state(0)
  onMount(() => {
    const t = setInterval(() => (seconds += 1), 1000)
    return () => clearInterval(t)
  })
</script>

{#if seconds < SLOW_S}
  <p>Loading projects…</p>
{:else}
  <p role="status">Mooring's server is not answering. Check that it is running. <span aria-hidden="true">({seconds} s)</span></p>
  <button type="button" class="btn" onclick={() => ((seconds = SLOW_S), app.retryNow())}>Retry now</button>
{/if}

<style>
  p {
    margin: 0;
  }
</style>
