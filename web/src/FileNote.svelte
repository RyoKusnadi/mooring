<script lang="ts">
  import { tick } from 'svelte'
  import { note } from './lib/filetransfer.svelte'

  let keep: HTMLButtonElement | undefined = $state()
  $effect(() => {
    if (note.ask) void tick().then(() => keep?.focus())
  })
</script>

{#if note.ask}
  <div class="note ask" role="group" aria-label={`${note.ask.name} already exists`}>
    <p>{note.ask.name} already exists in {note.ask.folder}.</p>
    <div class="buttons">
      <button type="button" class="btn small" onclick={() => note.choose('replace')}>Replace</button>
      <button type="button" class="btn small" bind:this={keep} onclick={() => note.choose('keep')}>Keep both</button>
      <button type="button" class="btn small" onclick={() => note.choose('cancel')}>Cancel</button>
    </div>
    <p class="sub">Replace moves the old file to the trash.</p>
  </div>
{:else if note.text}
  <p class="note" class:bad={note.bad} role={note.bad ? 'alert' : 'status'}>
    <span>{note.text}</span>
    <span class="acts">
      {#if note.undo}<button type="button" class="link" onclick={() => note.undo?.()}>Undo</button>{/if}
      <button type="button" class="link" onclick={() => note.clear()}>Dismiss</button>
    </span>
  </p>
{/if}

<style>
  .note {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-2);
    align-items: baseline;
    margin: 0;
    padding: var(--s-1) var(--s-3);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .note.bad {
    color: var(--danger);
  }
  .ask {
    display: grid;
    gap: var(--s-2);
    margin: var(--s-1) var(--s-2);
    border: 1px solid var(--border);
    border-radius: var(--r-l);
    background: var(--raised);
  }
  .ask p {
    margin: 0;
  }
  .sub {
    color: var(--muted);
  }
  .buttons {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .small {
    padding: 0 var(--s-2);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
  }
  .acts {
    display: inline-flex;
    gap: var(--s-3);
    white-space: nowrap;
  }
  @media (pointer: coarse) {
    .link {
      min-width: 44px;
      min-height: 44px;
      padding: 0 var(--s-2);
    }
  }
</style>
