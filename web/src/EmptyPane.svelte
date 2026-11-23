<script lang="ts">
  // The empty pane, and the old "blank" tab saved by earlier versions: say what can start here.
  import { actions } from './lib/actions'
  import { files } from './lib/filestore.svelte'
  import { app } from './lib/state.svelte'

  /** A saved blank tab to replace in place; empty for a pane with no tabs. */
  let { id = '' }: { id?: string } = $props()
  const gone = $derived(files.rootMissing(app.currentId))
</script>

<div class="note">
  <h2>Nothing is open here</h2>
  <p>{gone ? 'The project folder is missing, so a terminal or chat cannot start here. Put the folder back or choose another one.' : 'Start a terminal or a chat, or open a file of this project.'}</p>
  <div class="choices">
    <button type="button" class="btn primary" disabled={gone} onclick={() => (id ? actions.terminalHere(id) : actions.newTerminal())}>New terminal</button>
    <button type="button" class="btn" disabled={gone} onclick={() => (id ? actions.chatHere(id) : actions.newChat())}>New chat</button>
    <button type="button" class="btn" disabled={gone} onclick={() => (files.finder = 'file')}>Open file</button>
  </div>
</div>

<style>
  .note {
    display: grid;
    justify-items: center;
    gap: var(--s-3);
    max-width: 560px;
    margin: 0 auto;
    padding: var(--s-6) var(--s-4);
    text-align: center;
  }
  h2 {
    margin: 0;
    font-size: var(--fs-xl);
    line-height: var(--lh-xl);
    font-weight: var(--fw-semibold);
    text-wrap: balance;
  }
  p {
    margin: 0;
    color: var(--muted);
  }
  .choices {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: var(--s-2);
  }
</style>
