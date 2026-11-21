<script lang="ts">
  // Port forwards of one connection: a port on this computer that reaches a port as the host sees it.
  import { forwardProblem, localAddress, portOf } from './lib/sshfmt'
  import { ssh } from './lib/sshstore.svelte'

  let { conn, onBack }: { conn: string; onBack: () => void } = $props()

  let local = $state('0')
  let remoteHost = $state('localhost')
  let remotePort = $state('')
  let error = $state('')
  let busy = $state(false)
  const list = $derived(ssh.forwardsOf(conn))

  async function add(e: SubmitEvent) {
    e.preventDefault()
    const l = portOf(local)
    const r = portOf(remotePort)
    error = forwardProblem(l, remoteHost, r)
    if (error) return
    busy = true
    try {
      await ssh.forward(conn, l, remoteHost.trim(), r)
      remotePort = ''
    } catch (err) {
      error = err instanceof Error ? `Forward not started. Local port ${l || 'any'} may be taken, or the host could not reach ${remoteHost.trim()}:${r}. ${err.message}` : 'The forward could not start. Try again.'
    } finally {
      busy = false
    }
  }

  async function stop(id: string) {
    error = ''
    try {
      await ssh.stopForward(id)
    } catch (err) {
      error = err instanceof Error ? err.message : 'The forward could not be stopped. Try again.'
    }
  }
</script>

{#if list.length === 0}
  <p class="muted">No forwards on this connection.</p>
{:else}
  <ul data-testid="forwards">
    {#each list as f (f.id)}
      <li>
        <span class="what">
          <code>{localAddress(f.localPort)}</code>
          <span class="muted">reaches {f.remoteHost}:{f.remotePort} on the host</span>
        </span>
        <button type="button" class="btn" aria-label="Stop forward {localAddress(f.localPort)}" onclick={() => stop(f.id)}>Stop</button>
      </li>
    {/each}
  </ul>
{/if}

<form onsubmit={add} aria-label="Add a port forward">
  <div class="row">
    <div class="field">
      <label for="f-local">Local port</label>
      <input id="f-local" class="input" inputmode="numeric" bind:value={local} autocomplete="off" data-first />
    </div>
    <div class="field grow">
      <label for="f-host">Remote host</label>
      <input id="f-host" class="input mono" bind:value={remoteHost} autocomplete="off" spellcheck="false" autocapitalize="off" />
    </div>
    <div class="field">
      <label for="f-remote">Remote port</label>
      <input id="f-remote" class="input" inputmode="numeric" bind:value={remotePort} autocomplete="off" />
    </div>
  </div>
  <p class="help">Local port 0 picks any free port. The forward listens on 127.0.0.1 only and ends when the connection ends.</p>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="acts">
    <button type="button" class="btn" onclick={onBack}>Back to hosts</button>
    <button type="submit" class="btn primary" disabled={busy}>Add forward</button>
  </div>
</form>

<style>
  ul {
    margin: 0 0 var(--s-3);
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border);
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    padding: var(--s-2) 0;
    border-bottom: 1px solid var(--border);
  }
  .what {
    display: flex;
    flex-direction: column;
    min-width: 0;
    font-size: var(--fs-s);
  }
  code {
    font-size: var(--fs-m);
    user-select: all;
    overflow-wrap: anywhere;
  }
  .muted {
    margin: 0;
    color: var(--muted);
  }
  form {
    display: grid;
    gap: var(--s-2);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .row .field {
    width: 8em;
  }
  .row .grow {
    flex: 1;
    min-width: 12em;
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--s-2);
  }
</style>
