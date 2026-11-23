<script lang="ts">
  import { onMount } from 'svelte'
  import LoginPassword from './LoginPassword.svelte'
  import { login, passwordIsSet } from './lib/api'

  let { onSignIn }: { onSignIn: () => void } = $props()

  let token = $state('')
  let pending = $state(false)
  let error = $state('')
  let copied = $state(false)
  let tokenOpen = $state(false)
  // null until the server answers; a failed read shows the page as it is without a password.
  let passwordSet = $state<boolean | null>(null)
  onMount(() => {
    passwordIsSet()
      .then((set) => (passwordSet = set))
      .catch(() => (passwordSet = false))
  })

  // /login?code=... sends a refused link back here as /?link=used or /?link=invalid.
  const linkParam = new URLSearchParams(location.search).get('link')
  const notice =
    linkParam === 'used'
      ? 'That sign-in link was already used or has expired. Run mooring open for a new one.'
      : linkParam === 'wait'
        ? 'Too many sign-in attempts. Wait a minute and try again.'
      : linkParam === 'busy'
        ? 'Too many sign-in links in the last 5 minutes. Wait a few minutes and run mooring open again.'
      : linkParam === 'invalid'
        ? 'That sign-in link is not valid. Run mooring open for a new one.'
        : ''
  if (linkParam) history.replaceState(null, '', '/')

  async function copyCommand() {
    try {
      await navigator.clipboard.writeText('mooring open')
      copied = true
      setTimeout(() => (copied = false), 2000)
    } catch {
      copied = false
    }
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    if (!token.trim()) {
      error = 'Paste the login token first.'
      return
    }
    pending = true
    error = ''
    try {
      const result = await login(token)
      if (result.ok) {
        token = ''
        onSignIn()
      } else if (result.reason === 'too-many-attempts') {
        error = `Too many wrong tokens. Try again in ${result.retryAfterSeconds} seconds.`
      } else {
        error = 'That token does not match this server.'
      }
    } catch {
      error = 'Mooring\'s server is not answering. Check that it is running.'
    } finally {
      pending = false
    }
  }
</script>

<main class="login">
  <div class="card">
    <h1>Sign in to Mooring</h1>
    {#if notice}
      <p class="notice" role="status">{notice}</p>
    {/if}
    {#if passwordSet === null}
      <p class="help" role="status">Loading…</p>
    {:else}
      {#if passwordSet}
        <LoginPassword {onSignIn} />
        <p class="lead other">Or run <code>mooring open</code> in a terminal on this computer.</p>
      {:else}
        <p class="lead">Run <code>mooring open</code> in a terminal on this computer.</p>
      {/if}
      <div class="command">
        <code>mooring open</code>
        <button type="button" class="btn" onclick={copyCommand}>{copied ? 'Copied' : 'Copy'}</button>
        <span class="sr" aria-live="polite">{copied ? 'Copied' : ''}</span>
      </div>
      <details class="alt" bind:open={tokenOpen}>
        <summary>Use the login token instead</summary>
        <form onsubmit={submit}>
          <label for="token">Login token</label>
          <input
            id="token"
            class="input"
            name="token"
            type="password"
            autocomplete={passwordSet ? 'off' : 'current-password'}
            spellcheck="false"
            bind:value={token}
            oninput={() => (error = '')}
            aria-describedby={error ? 'token-help token-error' : 'token-help'}
            aria-invalid={error ? 'true' : undefined}
            aria-errormessage={error ? 'token-error' : undefined}
          />
          <p id="token-help" class="help">
            Run <code>mooring token</code> in a terminal on this computer and paste what it prints.
          </p>
          {#if error}
            <p id="token-error" class="error" role="alert">{error}</p>
          {/if}
          <button type="submit" class="btn" class:primary={!passwordSet} disabled={pending}>{pending ? 'Signing in…' : 'Sign in with token'}</button>
        </form>
      </details>
      {#if passwordSet === false}
        <p class="help">After you sign in, you can set a username and password in Settings.</p>
      {/if}
    {/if}
  </div>
</main>

<style>
  .login {
    display: grid;
    min-height: 100dvh;
    place-items: center;
    padding-top: calc(var(--s-5) + env(safe-area-inset-top, 0px));
    padding-bottom: calc(var(--s-5) + env(safe-area-inset-bottom, 0px));
    padding-left: max(var(--s-4), env(safe-area-inset-left, 0px));
    padding-right: max(var(--s-4), env(safe-area-inset-right, 0px));
  }
  .card {
    display: grid;
    gap: var(--s-3);
    width: min(100%, 360px);
    padding: var(--s-5);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-l);
  }
  h1 {
    margin: 0 0 var(--s-3);
    font-size: var(--fs-xl);
    line-height: var(--lh-xl);
  }
  .lead {
    margin: 0;
  }
  .other {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .notice {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-m);
  }
  .command {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-1) var(--s-1) var(--s-3);
    background: var(--raised);
    border: 1px solid var(--border);
    border-radius: var(--r-m);
  }
  .alt summary {
    display: flex;
    align-items: center;
    min-height: var(--control-h);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  form {
    display: grid;
    gap: var(--s-2);
  }
  label {
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .help,
  .error {
    margin: 0;
  }
  form .btn {
    margin-top: var(--s-2);
  }
</style>
