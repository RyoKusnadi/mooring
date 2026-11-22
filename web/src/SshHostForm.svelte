<script lang="ts">
  // Add or edit one saved host.
  import { untrack } from 'svelte'
  import type { Auth, HostDraft } from './lib/sshapi'
  import { AUTH_LABEL, hostProblem } from './lib/sshfmt'
  import { ssh } from './lib/sshstore.svelte'

  let { draft, onDone, onCancel }: { draft: HostDraft; onDone: () => void; onCancel: () => void } = $props()

  // The form is keyed by the host it edits, so it starts from the draft once.
  let f = $state<HostDraft>(untrack(() => ({ ...draft })))
  let port = $state(untrack(() => String(draft.port)))
  let error = $state('')
  let busy = $state(false)
  /** Set once the host is saved: the connection test runs, then its result line shows. */
  let saved = $state<{ id: string; testing: boolean; ok: boolean; message: string } | null>(null)
  const methods = Object.keys(AUTH_LABEL) as Auth[]

  async function test() {
    if (!saved) return
    saved = { ...saved, testing: true }
    const r = await ssh.test(saved.id)
    saved = { ...saved, testing: false, ...r }
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    const next = { ...f, name: f.name.trim(), address: f.address.trim(), user: f.user.trim(), root: f.root.trim(), keyPath: f.auth === 'key' ? f.keyPath.trim() : '', port: /^\d+$/.test(port.trim()) ? Number(port) : NaN }
    error = hostProblem(next)
    if (error) return
    busy = true
    try {
      const h = await ssh.saveHost(next)
      saved = { id: h.id, testing: true, ok: false, message: '' }
      await test()
    } catch (err) {
      error = err instanceof Error ? err.message : 'The host could not be saved. Try again.'
    } finally {
      busy = false
    }
  }
</script>

{#if saved}
  <div class="result" aria-label="Test of the connection">
    <p class="line" class:bad={!saved.testing && !saved.ok} role="status" data-first tabindex="-1">{saved.testing ? `Testing the connection to ${f.name}…` : saved.message}</p>
    {#if !saved.testing && !saved.ok}<p class="help">The host is saved. You can fix it later from the hosts list.</p>{/if}
    <div class="acts">
      {#if !saved.testing && !saved.ok}<button type="button" class="btn" onclick={test}>Test again</button>{/if}
      <button type="button" class="btn primary" disabled={saved.testing} onclick={onDone}>Done</button>
    </div>
  </div>
{:else}
<form onsubmit={submit} aria-label={draft.id ? 'Edit host' : 'Add host'}>
  <div class="field">
    <label for="h-name">Name</label>
    <input id="h-name" class="input" bind:value={f.name} autocomplete="off" placeholder="Build server" data-first />
  </div>
  <div class="row">
    <div class="field grow">
      <label for="h-address">Address</label>
      <input id="h-address" class="input mono" bind:value={f.address} autocomplete="off" spellcheck="false" autocapitalize="off" placeholder="build.example.com" />
    </div>
    <div class="field port">
      <label for="h-port">Port</label>
      <input id="h-port" class="input" inputmode="numeric" bind:value={port} autocomplete="off" />
    </div>
  </div>
  <div class="field">
    <label for="h-user">User</label>
    <input id="h-user" class="input mono" bind:value={f.user} autocomplete="off" spellcheck="false" autocapitalize="off" />
  </div>
  <div class="field">
    <label for="h-auth">Sign-in method</label>
    <span class="select"><select id="h-auth" class="input" bind:value={f.auth}>
      {#each methods as m (m)}<option value={m}>{AUTH_LABEL[m]}</option>{/each}
    </select></span>
    {#if f.auth === 'password'}<p class="help">Mooring asks for the password each time you connect. It can remember it on this computer if you allow it then.</p>{/if}
  </div>
  {#if f.auth === 'key'}
    <div class="field">
      <label for="h-key">Key file</label>
      <input id="h-key" class="input mono" bind:value={f.keyPath} autocomplete="off" spellcheck="false" placeholder="~/.ssh/id_ed25519" />
      <p class="help">The file must be inside your home folder on the computer Mooring runs on.</p>
    </div>
  {/if}
  <div class="field">
    <label for="h-root">Root folder</label>
    <input id="h-root" class="input mono" bind:value={f.root} autocomplete="off" spellcheck="false" placeholder="/home/me/work" />
    <p class="help">The remote folder the file browser starts in and stays inside. Empty means the home folder.</p>
  </div>
  <div class="field">
    <label class="check"><input type="checkbox" bind:checked={f.agentForward} /> Forward my SSH agent</label>
    <p class="help warn">Off is safer. With it on, anyone with admin rights on that host can use your keys while you are connected to it. Turn it on only for hosts you trust.</p>
  </div>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="acts">
    <button type="button" class="btn" onclick={onCancel}>Cancel</button>
    <button type="submit" class="btn primary" disabled={busy}>{busy ? 'Saving…' : draft.id ? 'Save and test' : 'Add and test'}</button>
  </div>
</form>
{/if}

<style>
  form {
    display: grid;
    gap: var(--s-3);
  }
  .row {
    display: flex;
    gap: var(--s-2);
  }
  .grow {
    flex: 1;
  }
  .port {
    width: 6.5em;
    flex: none;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-height: var(--control-h);
  }
  .check input {
    width: 18px;
    height: 18px;
    margin: 0;
  }
  .result {
    display: grid;
    gap: var(--s-3);
  }
  .line {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .line.bad {
    color: var(--danger);
    font-weight: 600;
  }
  .warn {
    color: var(--muted);
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--s-2);
  }
</style>
