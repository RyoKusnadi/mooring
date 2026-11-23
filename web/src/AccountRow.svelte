<script lang="ts">
  // One account: its name, login state and usage, and what can be done with it.
  import { tick } from 'svelte'
  import AccountUsage from './AccountUsage.svelte'
  import type { Account } from './lib/accountapi'
  import { signInTo } from './lib/accountactions'
  import { explain } from './lib/accountfmt'
  import { accounts } from './lib/accountstore.svelte'

  let { account: a, onGone, onSignIn }: { account: Account; onGone: () => void; onSignIn: () => void } = $props()

  const builtin = $derived(a.id.startsWith('default:'))
  let mode = $state<'' | 'rename' | 'remove'>('')
  let name = $state('')
  let busy = $state(false)
  let error = $state('')
  let nameBox: HTMLInputElement | undefined = $state()
  let removeBtn: HTMLButtonElement | undefined = $state()
  let renameBtn: HTMLButtonElement | undefined = $state()
  let keepBtn: HTMLButtonElement | undefined = $state()

  async function act(f: () => Promise<unknown>): Promise<boolean> {
    busy = true
    error = ''
    try {
      await f()
      return true
    } catch (e) {
      error = explain((e as { code?: string }).code ?? '', e instanceof Error ? e.message : '')
      return false
    } finally {
      busy = false
    }
  }

  async function ask(m: '' | 'rename' | 'remove') {
    mode = m
    error = ''
    if (m === 'rename') name = a.name
    await tick()
    if (m === 'rename') nameBox?.select()
    else if (m === 'remove') keepBtn?.focus()
    else (removeBtn ?? renameBtn)?.focus()
  }

  async function rename(e: Event) {
    e.preventDefault()
    if (await act(() => accounts.save({ id: a.id, agent: a.agent, name }))) await ask('')
  }

  async function remove() {
    if (await act(() => accounts.remove(a.id))) onGone()
  }
</script>

<li>
  <div class="what">
    <strong>{a.name}</strong>
    {#if a.isDefault && accounts.of(a.agent).length > 1}<span class="tag">Default for new projects</span>{/if}
    <span class="muted">{builtin ? "The agent's own login" : a.managed ? 'Folder managed by Mooring' : 'Your folder'}</span>
    {#if !builtin}<code>{a.dir}</code>{/if}
    <AccountUsage account={a} />
  </div>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if mode === 'rename'}
    <form class="acts" onsubmit={rename}>
      <label>Name <input class="input" bind:this={nameBox} bind:value={name} maxlength="60" required /></label>
      <button type="submit" class="btn primary" disabled={busy}>Save name</button>
      <button type="button" class="btn" data-keep onclick={() => ask('')}>Cancel</button>
    </form>
  {:else if mode === 'remove'}
    <div class="confirm" role="group" aria-label="Remove {a.name}">
      <span>Remove {a.name}? {a.managed ? 'Its folder moves to the removed folder in Mooring’s data, with the login inside.' : 'Your folder stays as it is.'} Projects using it go back to the default.</span>
      <button type="button" class="btn danger solid" disabled={busy} onclick={remove}>Remove</button>
      <button type="button" class="btn" bind:this={keepBtn} data-keep onclick={() => ask('')}>Keep</button>
    </div>
  {:else}
    <div class="acts">
      {#if !builtin}
        <button type="button" class="btn" aria-label="Sign in to {a.name}" disabled={busy} onclick={async () => (await act(() => signInTo(a.id))) && onSignIn()}>Sign in</button>
        <button type="button" class="btn" aria-label="Rename {a.name}" bind:this={renameBtn} onclick={() => ask('rename')}>Rename</button>
      {/if}
      {#if !a.isDefault}
        <button type="button" class="btn" aria-label="Make {a.name} the default" disabled={busy} onclick={() => act(() => accounts.setDefault(a.id))}>Make default</button>
      {/if}
      {#if !builtin}
        <button type="button" class="btn danger" aria-label="Remove {a.name}" bind:this={removeBtn} onclick={() => ask('remove')}>Remove</button>
      {/if}
    </div>
  {/if}
</li>

<style>
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
    align-items: flex-start;
    min-width: 0;
    gap: var(--s-1);
    font-size: var(--fs-s);
  }
  .what strong {
    font-size: var(--fs-m);
    overflow-wrap: anywhere;
  }
  .tag {
    padding: 0 var(--s-1);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-s);
    font-weight: var(--fw-semibold);
  }
  .muted {
    color: var(--muted);
  }
  code {
    max-width: 100%;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
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
  label {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  input {
    min-width: 0;
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
</style>
