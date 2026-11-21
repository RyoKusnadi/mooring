<script lang="ts">
  // One question from a host. It always says who is asking: user@address:port.
  import SshModal from './SshModal.svelte'
  import { refusal } from './lib/sshfmt'
  import { ssh } from './lib/sshstore.svelte'
  import type { SshPrompt } from './lib/sshapi'

  let { prompt }: { prompt: SshPrompt } = $props()

  let value = $state('')
  let remember = $state(false)
  let error = $state('')
  let busy = $state(false)

  // Trusting a new host and signing in to it are one step when the host uses a password.
  const withPassword = $derived(prompt.kind === 'hostkey' && ssh.hostOfPending(prompt.connId)?.auth === 'password')
  const secret = $derived(prompt.kind === 'password' || prompt.kind === 'passphrase' || (prompt.kind === 'question' && !prompt.echo))
  const title = $derived(
    prompt.kind === 'hostkey' ? 'Trust this host?' : prompt.kind === 'password' ? 'Password' : prompt.kind === 'passphrase' ? 'Key passphrase' : 'The host asks a question',
  )

  async function send(a: { value?: string; remember?: boolean; accept?: boolean; cancel?: boolean }) {
    if (busy) return
    error = ''
    busy = true
    try {
      await ssh.answer(prompt.promptId, a)
    } catch (e) {
      error = e instanceof Error ? e.message : 'The answer did not go through. Try again.'
    } finally {
      busy = false
    }
  }

  function trust() {
    if (withPassword && value) ssh.arm(prompt.connId, { value, remember: remember && ssh.canRemember })
    void send({ accept: true })
  }

  function submit(e: SubmitEvent) {
    e.preventDefault()
    void send({ value, remember: remember && ssh.canRemember })
  }
</script>

<SshModal {title} onClose={() => void send({ cancel: true })}>
  <p class="who">{prompt.host}</p>
  {#if prompt.kind === 'hostkey'}
    <p>This is the first time Mooring connects to this host. It sent this key.</p>
    <dl>
      <dt>Key type</dt>
      <dd>{prompt.keyType}</dd>
      <dt>Fingerprint</dt>
      <dd class="fp">{prompt.fingerprint}</dd>
    </dl>
    <p class="help">Compare the fingerprint with the one the owner of the host gave you. If it matches, trust it. Mooring saves the key and warns you if it changes.</p>
    <form onsubmit={(e) => { e.preventDefault(); trust() }}>
      {#if withPassword}
        <div class="field">
          <label for="ssh-trust-pw">Password for {prompt.host}</label>
          <input id="ssh-trust-pw" class="input" type="password" bind:value autocomplete="off" spellcheck="false" />
        </div>
        {#if ssh.canRemember}
          <label class="check"><input type="checkbox" bind:checked={remember} /> Remember on this computer</label>
          <p class="help">Kept in the secret store of the computer Mooring runs on. Never in Mooring's database.</p>
        {/if}
      {/if}
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <div class="acts">
        <button type="button" class="btn" data-first onclick={() => send({ accept: false })}>Don't connect</button>
        <button type="submit" class="btn primary" disabled={busy || (withPassword && !value)}>{withPassword ? 'Trust and sign in' : 'Trust and connect'}</button>
      </div>
    </form>
  {:else}
    <form onsubmit={submit}>
      {#if refusal(prompt)}<p class="error" role="alert">{refusal(prompt)}</p>{/if}
      <div class="field">
        <label for="ssh-answer">{prompt.kind === 'question' ? prompt.text || 'Answer' : prompt.kind === 'passphrase' ? `Passphrase for ${prompt.text || 'the key file'}` : `Password for ${prompt.host}`}</label>
        <input id="ssh-answer" class="input" type={secret ? 'password' : 'text'} bind:value autocomplete="off" spellcheck="false" data-first />
      </div>
      {#if ssh.canRemember && prompt.kind !== 'question'}
        <label class="check"><input type="checkbox" bind:checked={remember} /> Remember on this computer</label>
        <p class="help">Kept in the secret store of the computer Mooring runs on. Never in Mooring's database.</p>
      {/if}
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <div class="acts">
        <button type="button" class="btn" onclick={() => send({ cancel: true })}>Cancel</button>
        <button type="submit" class="btn primary" disabled={busy}>{prompt.kind === 'question' ? 'Send' : 'Sign in'}</button>
      </div>
    </form>
  {/if}
</SshModal>

<style>
  p {
    margin: 0 0 var(--s-3);
  }
  .who {
    font: var(--fs-m) var(--font-mono);
    overflow-wrap: anywhere;
  }
  dl {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: var(--s-1) var(--s-3);
    margin: 0 0 var(--s-3);
    padding: var(--s-2) var(--s-3);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-s);
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
  }
  .fp {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }
  form {
    display: grid;
    gap: var(--s-3);
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
  form .help,
  form .error {
    margin: 0;
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--s-2);
  }
</style>
