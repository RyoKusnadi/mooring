<script lang="ts">
  // A connection that stopped for a reason the user has to read. A changed host key gets the long form.
  import { tick } from 'svelte'
  import SshModal from './SshModal.svelte'
  import { signIn } from './lib/sshactions'
  import { hostLabel, readKeyChange } from './lib/sshfmt'
  import { ssh, type Failure } from './lib/sshstore.svelte'

  let { failure }: { failure: Failure } = $props()

  const host = $derived(ssh.host(failure.hostId))
  const changed = $derived(failure.code === 'HOST_KEY_CHANGED')
  const key = $derived(readKeyChange(failure.message))
  let forgotten = $state(false)
  let error = $state('')

  const TITLE: Record<string, string> = {
    HOST_KEY_CHANGED: 'The host key changed',
    HOST_UNREACHABLE: 'Cannot reach the host',
    AUTH_FAILED: 'The host refused the sign-in',
  }

  const close = () => (ssh.failure = null)

  async function forget() {
    if (!host) return
    error = ''
    try {
      await ssh.forgetKey(host)
      forgotten = true
      void tick().then(() => document.querySelector<HTMLElement>('[data-again]')?.focus())
    } catch (e) {
      error = e instanceof Error ? e.message : 'The saved key could not be removed. Try again.'
    }
  }

  function again() {
    // `failure` is a live view of the store, so read it before closing clears it.
    const hostId = failure.hostId
    close()
    void signIn(hostId)
  }
</script>

<SshModal title={TITLE[failure.code] ?? 'Could not connect'} onClose={close}>
  {#if host}<p class="who">{host.name}, {hostLabel(host)}</p>{/if}
  {#if changed}
    <p>Mooring refused to connect, because this host now shows a different key than the one saved for it. That can mean the host was reinstalled, or that someone is between you and it. Nothing was sent to the host.</p>
    {#if key.was}
      <dl>
        <dt>Saved key</dt>
        <dd class="fp">{key.was}</dd>
        <dt>Key it sent now</dt>
        <dd class="fp">{key.now}</dd>
      </dl>
    {/if}
    <p>{key.advice}</p>
    {#if forgotten}
      <p role="status">The saved key is gone. Connect again to see the new key and decide whether to trust it.</p>
    {/if}
  {:else}
    <p>{failure.message}</p>
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="acts">
    {#if changed && host?.savedKey === 'mooring' && !forgotten}
      <button type="button" class="btn danger" onclick={forget}>Forget the saved key</button>
    {/if}
    <button type="button" class="btn" data-first onclick={close}>Close</button>
    {#if host && (!changed || forgotten)}
      <button type="button" class="btn primary" data-again onclick={again}>Connect again</button>
    {/if}
  </div>
</SshModal>

<style>
  p {
    margin: 0 0 var(--s-3);
    overflow-wrap: anywhere;
  }
  .who {
    color: var(--muted);
    font-family: var(--font-mono);
  }
  dl {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 0;
    margin: 0 0 var(--s-3);
    padding: var(--s-2) var(--s-3);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-l);
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0 0 var(--s-2);
  }
  dd:last-child {
    margin-bottom: 0;
  }
  .fp {
    font-family: var(--font-mono);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--s-2);
  }
</style>
