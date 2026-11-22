<script lang="ts">
  import FolderBrowser from './FolderBrowser.svelte'
  import { app } from './lib/state.svelte'

  let { value = $bindable(), id, error = '' }: { value: string; id: string; error?: string } = $props()

  let browsing = $state(false)
</script>

<div class="field">
  <label for={id}>Parent folder</label>
  <div class="row">
    <input {id} class="input mono" bind:value autocomplete="off" spellcheck="false" aria-invalid={error ? 'true' : undefined} aria-describedby={error ? id + '-error' : undefined} />
    <button type="button" class="btn" aria-expanded={browsing} onclick={() => (browsing = !browsing)}>{browsing ? 'Hide folders' : 'Browse'}</button>
  </div>
  {#if error}<p class="error" id={id + '-error'} role="alert">{error}</p>{/if}
  {#if browsing}
    <div class="browse">
      <FolderBrowser
        start={value.trim() || app.home}
        actionLabel="Use this folder"
        onPick={(p) => {
          value = p
          browsing = false
        }}
      />
    </div>
  {/if}
</div>

<style>
  .row {
    display: flex;
    gap: var(--s-2);
  }
  .browse {
    padding-top: var(--s-2);
  }
</style>
