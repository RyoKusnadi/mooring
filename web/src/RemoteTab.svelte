<script lang="ts">
  // A tab on an SSH host: the folder browser or one remote file. Without a live connection it offers Reconnect.
  import FileLazy from './FileLazy.svelte'
  import RemoteLazy from './RemoteLazy.svelte'
  import { signIn } from './lib/sshactions'
  import { hostLabel } from './lib/sshfmt'
  import { ssh } from './lib/sshstore.svelte'
  import type { Tab } from './lib/layout'

  let { tab }: { tab: Tab } = $props()

  const host = $derived(tab.host ? ssh.host(tab.host) : undefined)
  const conn = $derived(tab.host ? ssh.connFor(tab.host) : undefined)
  const connecting = $derived(!!tab.host && !!ssh.connecting[tab.host])
  const reason = $derived(tab.host ? ssh.closedHost[tab.host] : '')
</script>

{#if conn && tab.host}
  {#if tab.kind === 'remote'}
    <RemoteLazy conn={conn.id} host={tab.host} />
  {:else if tab.path !== undefined}
    <!-- A new connection gets a fresh editor; unsaved edits come back from the tab's saved state. -->
    {#key conn.id}<FileLazy id={tab.id} path={tab.path} remote={conn.id} />{/key}
  {/if}
{:else if !ssh.loaded}
  <p class="note" role="status">Loading…</p>
{:else if !host}
  <p class="note">This host was removed from Mooring. Close this tab, or add the host again.</p>
{:else}
  <div class="note">
    <p role="status">
      {#if connecting}Connecting to {host.name}…{:else}Not connected to {host.name} ({hostLabel(host)}).{#if reason}{' '}{reason}{/if}{/if}
    </p>
    <button type="button" class="btn primary" disabled={connecting} onclick={() => signIn(host.id)}>Reconnect</button>
  </div>
{/if}

<style>
  .note {
    display: grid;
    justify-items: start;
    gap: var(--s-3);
    max-width: 52ch;
    margin: 0;
    padding: var(--s-5) var(--s-4);
  }
  .note p,
  p.note {
    margin: 0;
    color: var(--muted);
  }
</style>
