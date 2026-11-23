<script lang="ts">
  // Sign-in section of Settings: set, change or remove the username and password for the login page.
  // The server keeps only a hash. Changing the password does not sign out other browsers.
  import { onMount } from 'svelte'
  import { checkNew, fetchPasswordStatus, signOutOthers, type PasswordStatus } from './lib/password'
  import { settings } from './lib/settingsstore.svelte'

  let status = $state<PasswordStatus | null>(null)
  let loadError = $state('')
  let username = $state('')
  let current = $state('')
  let next = $state('')
  let again = $state('')
  let removeCurrent = $state('')
  let error = $state('')
  let removeError = $state('')
  let othersCurrent = $state('')
  let othersError = $state('')
  let done = $state('')
  let pending = $state(false)
  const uid = $props.id()

  onMount(async () => {
    try {
      status = await fetchPasswordStatus()
      username = status.username
    } catch {
      loadError = 'Could not read the sign-in state. Check that Mooring is running.'
    }
  })

  function clear() {
    current = next = again = removeCurrent = othersCurrent = ''
  }

  async function run(name: 'password.set' | 'password.remove', args: object, fail: (m: string) => void, message: string) {
    pending = true
    done = ''
    try {
      status = await settings.cmd<PasswordStatus>(name, args)
      username = status.username
      clear()
      done = message
    } catch (e) {
      fail(e instanceof Error ? e.message : 'Could not save. Try again.')
      current = next = again = removeCurrent = ''
    } finally {
      pending = false
    }
  }

  function save(event: SubmitEvent) {
    event.preventDefault()
    error = removeError = othersError = ''
    const problem = checkNew(username, next, again)
    if (problem) {
      error = problem
      return
    }
    if (status?.set && !current) {
      error = 'Enter the current password.'
      return
    }
    void run('password.set', { current, username, password: next }, (m) => (error = m), status?.set ? 'Password changed.' : 'Password set. The login page now asks for it.')
  }

  function remove(event: SubmitEvent) {
    event.preventDefault()
    error = removeError = othersError = ''
    if (!removeCurrent) {
      removeError = 'Enter the current password.'
      return
    }
    void run('password.remove', { current: removeCurrent }, (m) => (removeError = m), 'Password removed. The login page asks for the token or a sign-in link again.')
  }

  async function others(event: SubmitEvent) {
    event.preventDefault()
    error = removeError = othersError = done = ''
    if (!othersCurrent) {
      othersError = 'Enter the current password.'
      return
    }
    pending = true
    try {
      othersError = await signOutOthers(othersCurrent)
      if (!othersError) done = 'Other browsers are signed out. This one stays signed in.'
    } catch {
      othersError = 'Mooring\'s server is not answering. Check that it is running.'
    } finally {
      othersCurrent = ''
      pending = false
    }
  }
</script>

{#if loadError}
  <p class="error" role="alert">{loadError}</p>
{:else if status}
  <p class="help lead">
    {#if status.set}
      The login page asks for the username <strong>{status.username}</strong> and its password. The token and <code>mooring open</code> still work.
    {:else}
      No password is set. The login page asks for the token or a sign-in link. Set a username and password to sign in with those instead.
    {/if}
  </p>

  <form onsubmit={save}>
    <h3>{status.set ? 'Change password' : 'Set a password'}</h3>
    <label for="{uid}-user">Username</label>
    <input id="{uid}-user" class="input" autocomplete="username" autocapitalize="off" spellcheck="false" bind:value={username} oninput={() => (error = '')} />
    {#if status.set}
      <label for="{uid}-current">Current password</label>
      <input id="{uid}-current" class="input" type="password" autocomplete="current-password" bind:value={current} oninput={() => (error = '')} />
    {/if}
    <label for="{uid}-new">New password</label>
    <input id="{uid}-new" class="input" type="password" autocomplete="new-password" bind:value={next} oninput={() => (error = '')} aria-describedby="{uid}-rule" />
    <span id="{uid}-rule" class="help">At least 8 characters.</span>
    <label for="{uid}-again">New password again</label>
    <input id="{uid}-again" class="input" type="password" autocomplete="new-password" bind:value={again} oninput={() => (error = '')} aria-invalid={error ? 'true' : undefined} />
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="actions">
      <button type="submit" class="btn primary" disabled={pending}>{status.set ? 'Change password' : 'Set password'}</button>
    </div>
  </form>

  {#if status.set}
    <form onsubmit={remove}>
      <h3>Remove password</h3>
      <label for="{uid}-rm">Password to confirm removal</label>
      <input id="{uid}-rm" class="input" type="password" autocomplete="current-password" bind:value={removeCurrent} oninput={() => (removeError = '')} aria-invalid={removeError ? 'true' : undefined} />
      {#if removeError}<p class="error" role="alert">{removeError}</p>{/if}
      <div class="actions">
        <button type="submit" class="btn" disabled={pending}>Remove password</button>
      </div>
    </form>
  {/if}

  {#if status.set}
    <form onsubmit={others}>
      <h3>Sign out other browsers</h3>
      <span class="help">Ends every other sign-in and every sign-in link, like <code>mooring token -new</code>. This browser stays signed in.</span>
      <label for="{uid}-others">Password to confirm sign-out</label>
      <input id="{uid}-others" class="input" type="password" autocomplete="current-password" bind:value={othersCurrent} oninput={() => (othersError = '')} aria-invalid={othersError ? 'true' : undefined} />
      {#if othersError}<p class="error" role="alert">{othersError}</p>{/if}
      <div class="actions">
        <button type="submit" class="btn" disabled={pending}>Sign out other browsers</button>
      </div>
    </form>
  {/if}

  <p class="status" role="status">{done}</p>
  <p class="help">
    Changing or removing the password does not sign out other browsers. Use the button above, or run <code>mooring token -new</code> in a terminal.
    If you forget the password, run <code>mooring password</code> in a terminal on this computer to set a new one, or <code>mooring password -remove</code> to remove it.
  </p>
{:else}
  <p class="help" role="status">Loading…</p>
{/if}

<style>
  form {
    display: grid;
    gap: var(--s-2);
    max-width: 360px;
    margin-bottom: var(--s-4);
  }
  h3 {
    margin: 0;
    font-size: var(--fs-m);
  }
  label {
    margin-top: var(--s-1);
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .lead {
    margin-bottom: var(--s-4);
  }
  .actions {
    margin-top: var(--s-2);
  }
  .status {
    min-height: 1.2em;
    margin: 0 0 var(--s-3);
    color: var(--success);
  }
</style>
