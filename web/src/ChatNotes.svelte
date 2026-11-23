<script lang="ts">
  // The lines under the message box: a problem (read out at once) and a note that explains the box.
  let { id, error, note, urgent }: { id: string; error: string; note: string; urgent: boolean } = $props()
</script>

{#if error}<p class="error" role="alert" id={'err-' + id}>{error}</p>{/if}
{#if note}<p class="help" role={urgent ? 'alert' : 'status'} id={'note-' + id}>{note}</p>{/if}
