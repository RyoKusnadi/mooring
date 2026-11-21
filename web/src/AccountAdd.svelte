<script lang="ts">
  // Adds an account for one agent: a folder Mooring manages, or a folder you already have.
  import { tick } from 'svelte'
  import { explain } from './lib/accountfmt'
  import { accounts } from './lib/accountstore.svelte'

  let { agent, agentName, onDone }: { agent: string; agentName: string; onDone: () => void } = $props()

  const uid = $props.id()
  let name = $state('')
  let own = $state(false)
  let dir = $state('')
  let busy = $state(false)
  let error = $state('')
  let nameBox: HTMLInputElement | undefined = $state()

  $effect(() => void tick().then(() => nameBox?.focus()))

  async function submit(e: Event) {
    e.preventDefault()
    busy = true
    error = ''
    try {
      await accounts.save({ agent, name, ...(own ? { dir: dir.trim() } : {}) })
      onDone()
    } catch (err) {
      error = explain((err as { code?: string }).code ?? '', err instanceof Error ? err.message : '')
    } finally {
      busy = false
    }
  }
</script>

<form onsubmit={submit} aria-label="Add a {agentName} account">
  <div class="field">
    <label for="{uid}-name">Name</label>
    <input id="{uid}-name" class="input" bind:this={nameBox} bind:value={name} maxlength="60" required placeholder="Work" />
  </div>
  <fieldset>
    <legend>Settings folder</legend>
    <label><input type="radio" name="{uid}-where" checked={!own} onchange={() => (own = false)} /> Mooring makes a folder</label>
    <label><input type="radio" name="{uid}-where" checked={own} onchange={() => (own = true)} /> Use a folder I already have</label>
  </fieldset>
  {#if own}
    <div class="field">
      <label for="{uid}-dir">Folder path</label>
      <input id="{uid}-dir" class="input mono" bind:value={dir} required placeholder="/Users/you/.claude-work" />
      <span class="hint">An existing folder inside your home folder. It cannot be the agent's own folder or another account's.</span>
    </div>
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="acts">
    <button type="submit" class="btn primary" disabled={busy}>Add account</button>
    <button type="button" class="btn" data-keep onclick={onDone}>Cancel</button>
  </div>
  <p class="hint">A new account starts signed out. After adding it, use Sign in on its row.</p>
</form>

<style>
  form {
    display: grid;
    gap: var(--s-3);
    margin: var(--s-2) 0 var(--s-3);
  }
  fieldset {
    display: grid;
    gap: var(--s-1);
    margin: 0;
    padding: 0;
    border: 0;
  }
  legend {
    padding: 0;
    font-weight: 600;
  }
  fieldset label {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-height: var(--control-h);
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .hint {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-s);
  }
</style>
