<script lang="ts">
  // Sign in to a remote tool server. The sign-in page opens in a new tab. When the browser runs on the
  // same machine as Mooring, the server finishes by itself. Otherwise the page it ends on does not load,
  // and its address is pasted here.
  import { onMount } from 'svelte'
  import { CommandError } from './lib/events'
  import { explain, safeUrl } from './lib/toolfmt'
  import { tools } from './lib/toolstore.svelte'

  let { id, onDone }: { id: string; onDone: () => void } = $props()

  let url = $state('')
  let paste = $state('')
  let error = $state('')
  let busy = $state(false)
  let loading = $state(true)
  const s = $derived(tools.get(id))
  const done = $derived(s?.signIn === 'signedIn')
  const failed = $derived(s?.signIn === 'failed')

  async function begin() {
    loading = true
    error = ''
    try {
      url = safeUrl((await tools.signIn(id)).url)
      if (!url) error = 'The server did not give a sign-in page. Try again.'
    } catch (e) {
      error = explain(e instanceof CommandError ? e.code : '', e instanceof Error ? e.message : 'Could not start the sign-in. Try again.')
    } finally {
      loading = false
    }
  }
  onMount(() => void begin())

  async function finish(e: SubmitEvent) {
    e.preventDefault()
    if (!paste.trim()) {
      error = 'Paste the address of the page the sign-in ended on.'
      return
    }
    busy = true
    error = ''
    try {
      await tools.signInFinish(id, paste.trim())
      paste = ''
    } catch (err) {
      error = explain(err instanceof CommandError ? err.code : '', err instanceof Error ? err.message : 'The sign-in did not finish. Open the sign-in page again.')
    } finally {
      busy = false
    }
  }
</script>

<div class="wrap" data-first tabindex="-1">
  {#if done}
    <p role="status">Signed in to {s?.name}.</p>
    <div class="acts"><button type="button" class="btn primary" onclick={onDone}>Back to list</button></div>
  {:else}
    <ol>
      <li>
        {#if url}
          <a class="btn primary" href={url} target="_blank" rel="noopener noreferrer">Open the sign-in page</a>
          <p class="help">It opens in a new tab.</p>
        {:else if loading}
          <p class="help" role="status">Preparing the sign-in…</p>
        {/if}
      </li>
      <li>
        {#if failed}
          <p class="error" role="alert">{error || s?.note || explain('SIGN_IN_FAILED', '')}</p>
          <button type="button" class="btn" disabled={loading} onclick={begin}>{loading ? 'Preparing…' : 'Start the sign-in again'}</button>
        {:else}
          <p>On the same computer as Mooring, the sign-in finishes by itself and this screen updates.</p>
        {/if}
      </li>
      <li>
        <p>On another computer, the sign-in ends on a page that does not load. Copy its address from the address bar and paste it here.</p>
        <form onsubmit={finish}>
          <div class="field">
            <label for="ts-paste">Address of the page it ended on</label>
            <input id="ts-paste" class="input mono" bind:value={paste} autocomplete="off" spellcheck="false" autocapitalize="off" inputmode="url" placeholder="…/callback?code=…" />
          </div>
          <button type="submit" class="btn" disabled={busy || !url}>{busy ? 'Finishing…' : 'Finish sign-in'}</button>
        </form>
      </li>
    </ol>
    {#if error && !failed}<p class="error" role="alert">{error}</p>{/if}
    <div class="acts"><button type="button" class="btn" onclick={onDone}>Cancel</button></div>
  {/if}
</div>

<style>
  .wrap {
    display: grid;
    gap: var(--s-3);
  }
  p {
    margin: 0 0 var(--s-2);
  }
  ol {
    display: grid;
    gap: var(--s-3);
    margin: 0;
    padding-left: var(--s-4);
  }
  a.btn {
    text-decoration: none;
  }
  form {
    display: grid;
    justify-items: start;
    gap: var(--s-2);
  }
  form .field {
    width: 100%;
  }
  .acts {
    display: flex;
    justify-content: flex-end;
  }
</style>
