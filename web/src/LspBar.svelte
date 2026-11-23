<script lang="ts">
  import { lsp, lspKey } from './lib/lspstore.svelte'
  import { splitPlace, type LspServer } from './lib/lspmatch'
  import { app } from './lib/state.svelte'

  let { server, path }: { server: LspServer; path: string } = $props()

  let busy = $state(false)
  let problem = $state('')

  const key = $derived(lspKey(app.currentId, splitPlace(path).place, server.id))
  const conn = $derived(lsp.conn[key])
  const output = $derived(lsp.crashes[key])
  // The crash and stop bars can be hidden until the connection changes again. The choice lives in
  // the store, so it survives this bar being drawn again (a reload of the file, a tab switch).
  const phase = $derived(conn ?? 'none')
  const hidden = $derived((lsp.dismissed[server.id] && server.state !== 'ready') || lsp.shut[key] === phase)

  async function act(run: () => Promise<unknown>) {
    busy = true
    problem = ''
    try {
      await run()
    } catch (e) {
      problem = e instanceof Error ? e.message : 'That did not work. Try again.'
    } finally {
      busy = false
    }
  }

  /** The program's file name and its arguments, without the folder it sits in. */
  const short = (cmd: string[]) => [cmd[0]?.slice(cmd[0].lastIndexOf('/') + 1), ...cmd.slice(1)].join(' ')
  const startAgain = () => import('./lib/lspbind').then((m) => m.restart(key))
</script>

{#if !hidden}
  <div class="over">
    {#if server.state === 'not_installed'}
      <div class="lspbar" role="group" aria-label="{server.name} language server">
        <span class="msg">
          {#if server.installable}{server.name} is not installed. Install it for completions, errors and go to definition in {server.endings.join(', ')} files.{:else}{server.name} is not installed. {server.installHint}{/if}
        </span>
        <span class="acts">
          {#if server.installable}<button type="button" class="btn" disabled={busy} onclick={() => act(() => lsp.install(server.id))}>Install {server.name}</button>{/if}
          <button type="button" class="btn ghost" onclick={() => (lsp.dismissed[server.id] = true)}>Not now</button>
        </span>
      </div>
    {:else if server.state === 'needs_consent'}
      <div class="lspbar" role="group" aria-label="{server.name} language server">
        <span class="msg">
          Mooring wants to run <code class="short">{short(server.command)}</code> for {server.endings.join(', ')} files.
          <details class="exact"><summary>Show full command</summary><code>{server.command.join(' ')}</code></details>
        </span>
        <span class="acts">
          <button type="button" class="btn primary" disabled={busy} onclick={() => act(() => lsp.allow(server.id, server.command))}>Allow and start</button>
          <button type="button" class="btn ghost" onclick={() => (lsp.dismissed[server.id] = true)}>Not now</button>
        </span>
      </div>
    {:else if conn === 'failed' || (output && conn !== 'open')}
      <div class="lspbar crash" role="alert">
        <span class="msg">{server.name} stopped.{#if conn === 'failed'} It crashed three times in two minutes.{/if}</span>
        {#if output}<pre>{output}</pre>{/if}
        <span class="acts">
          <button type="button" class="btn" onclick={startAgain}>Start again</button>
          <button type="button" class="btn ghost" onclick={() => (lsp.shut[key] = phase)}>Hide</button>
        </span>
      </div>
    {:else if conn === 'stopped'}
      <div class="lspbar" role="status">
        <span class="msg">{server.name} was stopped.</span>
        <span class="acts">
          <button type="button" class="btn" onclick={startAgain}>Start again</button>
          <button type="button" class="btn ghost" onclick={() => (lsp.shut[key] = phase)}>Hide</button>
        </span>
      </div>
    {/if}
    {#if problem}<p class="problem" role="alert">{problem}</p>{/if}
  </div>
{/if}

<style>
  /* A row above the editor: it pushes the code down, so it never covers a line. Installing and reconnecting show in the status bar. */
  .over {
    flex: none;
    max-height: 40%;
    overflow: auto;
  }
  .lspbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
    background: var(--surface);
    font-size: var(--fs-s);
  }
  .msg {
    flex: 1 1 24ch;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  code,
  pre {
    font-family: var(--font-mono);
  }
  pre {
    flex: 1 1 100%;
    max-height: 7em;
    margin: 0;
    overflow: auto;
    color: var(--muted);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .acts {
    display: flex;
    flex: none;
    gap: var(--s-2);
  }
  .exact {
    margin-top: var(--s-1);
    color: var(--muted);
  }
  .exact summary {
    display: flex;
    align-items: center;
    min-height: var(--control-h);
    cursor: pointer;
  }
  .exact code {
    display: block;
    margin-top: var(--s-1);
  }
  @media (max-width: 900px), (pointer: coarse) {
    .exact summary,
    .lspbar .btn {
      min-height: 44px;
    }
    .lspbar .btn {
      flex: none;
    }
  }
  .crash .msg {
    color: var(--danger);
    font-weight: var(--fw-semibold);
  }
  .problem {
    margin: 0;
    padding: var(--s-1) var(--s-3);
    background: var(--surface);
    color: var(--danger);
    font-size: var(--fs-s);
    border-bottom: 1px solid var(--border);
  }
</style>
