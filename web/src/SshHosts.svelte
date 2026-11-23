<script lang="ts">
  // The "SSH hosts" dialog: saved hosts, each with what it can open, plus add, edit, import and forwards.
  import { onMount, tick } from 'svelte'
  import SshForwards from './SshForwards.svelte'
  import SshHostForm from './SshHostForm.svelte'
  import SshImport from './SshImport.svelte'
  import SshModal from './SshModal.svelte'
  import { actions } from './lib/actions'
  import type { Host, HostDraft } from './lib/sshapi'
  import { draftOf, emptyDraft, hostLabel } from './lib/sshfmt'
  import { ssh } from './lib/sshstore.svelte'

  let { onClose }: { onClose: () => void } = $props()

  type View = { v: 'list' } | { v: 'form'; draft: HostDraft } | { v: 'import' } | { v: 'forwards'; conn: string }
  let view = $state<View>({ v: 'list' })
  let removing = $state('')
  let error = $state('')
  let box: HTMLElement | undefined = $state()
  let seen = ''

  // A new view takes the focus to its own first control.
  $effect(() => {
    const v = view.v
    if (seen && v !== seen) void tick().then(() => box?.querySelector<HTMLElement>('[data-first]')?.focus())
    seen = v
  })

  onMount(() => {
    void ssh.load().catch(() => (error = 'Could not read the saved hosts. Check that Mooring is running.'))
  })

  const title = $derived(view.v === 'form' ? (view.draft.id ? 'Edit host' : 'Add host') : view.v === 'import' ? 'Import hosts' : view.v === 'forwards' ? `Port forwards on ${ssh.hostOfConn(view.conn)?.name ?? 'the host'}` : 'SSH hosts')

  function open(h: Host, what: 'terminalOn' | 'filesOn') {
    ssh.dialog = false
    void actions[what](h.id)
  }

  async function remove(h: Host) {
    error = ''
    try {
      await ssh.removeHost(h.id)
      removing = ''
      void tick().then(() => box?.querySelector<HTMLElement>('[data-first]')?.focus())
      removing = ''
    } catch (e) {
      error = e instanceof Error ? e.message : 'The host could not be removed. Try again.'
    }
  }

  async function disconnect(connId: string) {
    error = ''
    try {
      await ssh.disconnect(connId)
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not disconnect. Try again.'
    }
  }

  const stateOf = (h: Host) => (ssh.connFor(h.id) ? 'Connected' : ssh.connecting[h.id] ? 'Connecting…' : '')
</script>

<SshModal {title} {onClose} wide>
  <div bind:this={box}>
  {#if view.v === 'form'}
    {#key view.draft.id ?? 'new'}<SshHostForm draft={view.draft} onDone={() => (view = { v: 'list' })} onCancel={() => (view = { v: 'list' })} />{/key}
  {:else if view.v === 'import'}
    <SshImport onDone={() => (view = { v: 'list' })} onCancel={() => (view = { v: 'list' })} />
  {:else if view.v === 'forwards'}
    <SshForwards conn={view.conn} onBack={() => (view = { v: 'list' })} />
  {:else}
    <div class="top">
      <button type="button" class="btn primary" data-first onclick={() => (view = { v: 'form', draft: emptyDraft() })}>Add host</button>
      <button type="button" class="btn" onclick={() => (view = { v: 'import' })}>Import from SSH config</button>
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    {#if !ssh.loaded && !error}
      <p class="muted" role="status">Loading…</p>
    {:else if ssh.hosts.length === 0}
      <p class="muted">No hosts yet. Add one, or import the hosts in your SSH config.</p>
    {:else}
      <ul>
        {#each ssh.hosts as h (h.id)}
          {@const conn = ssh.connFor(h.id)}
          <li>
            <div class="what">
              <strong>{h.name}</strong>
              <code>{hostLabel(h)}</code>
              <span class="muted">{[stateOf(h), h.remembered ? 'Password remembered' : '', h.agentForward ? 'Agent forwarding on' : ''].filter(Boolean).join(', ')}</span>
            </div>
            {#if removing === h.id}
              <div class="confirm" role="group" aria-label="Remove {h.name}">
                <span>Remove {h.name}? Open connections stay until closed.</span>
                <button type="button" class="btn danger solid" onclick={() => remove(h)}>Remove</button>
                <button type="button" class="btn" onclick={() => (removing = '')}>Keep</button>
              </div>
            {:else}
              <div class="acts">
                <button type="button" class="btn" aria-label="Open a terminal on {h.name}" onclick={() => open(h, 'terminalOn')}>Terminal</button>
                <button type="button" class="btn" aria-label="Browse files on {h.name}" onclick={() => open(h, 'filesOn')}>Files</button>
                {#if conn}
                  <button type="button" class="btn" aria-label="Port forwards on {h.name}" onclick={() => (view = { v: 'forwards', conn: conn.id })}>Forwards</button>
                  <button type="button" class="btn" aria-label="Disconnect {h.name}" onclick={() => disconnect(conn.id)}>Disconnect</button>
                {/if}
                <button type="button" class="btn" aria-label="Edit {h.name}" onclick={() => (view = { v: 'form', draft: draftOf(h) })}>Edit</button>
                <button type="button" class="btn danger" aria-label="Remove {h.name}" onclick={() => (removing = h.id)}>Remove</button>
              </div>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
  </div>
</SshModal>

<style>
  .top {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
    margin-bottom: var(--s-3);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border);
  }
  li {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3) 0;
    border-bottom: 1px solid var(--border);
  }
  .what {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: var(--s-1);
    font-size: var(--fs-s);
  }
  .what strong {
    font-size: var(--fs-m);
  }
  code {
    overflow-wrap: anywhere;
  }
  .muted {
    margin: 0;
    color: var(--muted);
  }
  .acts,
  .confirm {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
  }
  .confirm span {
    flex: 1 1 100%;
  }
</style>
