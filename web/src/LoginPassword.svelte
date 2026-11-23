<script lang="ts">
  // The username and password form on the login page. It shows once a password has been set in Settings.
  import { loginWithPassword } from './lib/api'

  let { onSignIn }: { onSignIn: () => void } = $props()

  let username = $state('')
  let password = $state('')
  let pending = $state(false)
  let error = $state('')
  let passwordField: HTMLInputElement | undefined = $state()

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    if (!username.trim() || !password) {
      error = 'Enter your username and password.'
      return
    }
    pending = true
    error = ''
    try {
      const result = await loginWithPassword(username, password)
      if (result.ok) {
        password = ''
        onSignIn()
        return
      }
      password = ''
      error =
        result.reason === 'too-many-attempts'
          ? `Too many wrong attempts. Try again in ${result.retryAfterSeconds} seconds.`
          : result.reason === 'password-file'
            ? 'The password file could not be read. Sign in with mooring open and set the password again.'
            : 'Wrong username or password.'
      passwordField?.focus()
    } catch {
      password = ''
      error = "Mooring's server is not answering. Check that it is running."
    } finally {
      pending = false
    }
  }
</script>

<form onsubmit={submit}>
  <label for="login-username">Username</label>
  <input
    id="login-username"
    class="input"
    name="username"
    autocomplete="username"
    autocapitalize="off"
    spellcheck="false"
    bind:value={username}
    oninput={() => (error = '')}
  />
  <label for="login-password">Password</label>
  <input
    id="login-password"
    class="input"
    name="password"
    type="password"
    autocomplete="current-password"
    bind:this={passwordField}
    bind:value={password}
    oninput={() => (error = '')}
    aria-describedby={error ? 'login-error' : undefined}
    aria-invalid={error ? 'true' : undefined}
  />
  {#if error}
    <p id="login-error" class="error" role="alert">{error}</p>
  {/if}
  <button type="submit" class="btn primary" disabled={pending}>{pending ? 'Signing in…' : 'Sign in'}</button>
</form>

<style>
  form {
    display: grid;
    gap: var(--s-2);
  }
  label {
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  button {
    margin-top: var(--s-2);
  }
</style>
