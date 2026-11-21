<script lang="ts">
  // Add or edit a tool server the user types: a local command, or a remote address.
  // Values for environment variables and headers are not asked for here; the consent screen does that.
  import { untrack } from 'svelte'
  import type { ServerDraft } from './lib/toolapi'
  import { draftProblem, splitArgs, varsFromNames } from './lib/toolfmt'

  let { draft, onBack, onNext }: { draft: ServerDraft; onBack: () => void; onNext: (d: ServerDraft) => void } = $props()

  // The form is keyed by the server it edits, so it starts from the draft once.
  let f = $state<ServerDraft>(untrack(() => ({ ...draft })))
  let argText = $state(untrack(() => draft.args.join('\n')))
  let names = $state(untrack(() => (draft.kind === 'stdio' ? draft.env : draft.headers).map((v) => v.name).join(' ')))
  let error = $state('')
  const remote = $derived(f.kind !== 'stdio')

  function submit(e: SubmitEvent) {
    e.preventDefault()
    const next: ServerDraft = remote
      ? { ...f, name: f.name.trim(), url: f.url.trim(), command: '', args: [], env: [], headers: varsFromNames(names, draft.headers) }
      : { ...f, name: f.name.trim(), command: f.command.trim(), url: '', allowPrivate: false, args: splitArgs(argText), headers: [], env: varsFromNames(names, draft.env) }
    error = draftProblem(next)
    if (!error) onNext(next)
  }
</script>

<form onsubmit={submit} aria-label={draft.id ? 'Edit tool server' : 'Add your own tool server'}>
  <fieldset>
    <legend>How it runs</legend>
    <label class="check"><input type="radio" name="tf-kind" value="stdio" bind:group={f.kind} /> A command on this computer</label>
    <label class="check"><input type="radio" name="tf-kind" value="http" bind:group={f.kind} /> A remote address (http)</label>
    <label class="check"><input type="radio" name="tf-kind" value="sse" bind:group={f.kind} /> A remote address (sse, older)</label>
  </fieldset>
  <div class="field">
    <label for="tf-name">Name</label>
    <input id="tf-name" class="input" bind:value={f.name} autocomplete="off" placeholder="Notes" data-first />
  </div>
  {#if remote}
    <div class="field">
      <label for="tf-url">Address</label>
      <input id="tf-url" class="input mono" bind:value={f.url} autocomplete="off" spellcheck="false" autocapitalize="off" inputmode="url" placeholder="https://example.com/mcp" />
    </div>
    <div class="field">
      <label for="tf-names">Header names (optional)</label>
      <input id="tf-names" class="input mono" bind:value={names} autocomplete="off" spellcheck="false" autocapitalize="off" aria-describedby="tf-names-help" placeholder="X-Api-Key" />
      <p class="help" id="tf-names-help">Separate with spaces. You enter the values on the next screen, where they go to the secret store.</p>
    </div>
    <div class="field">
      <label class="check"><input type="checkbox" bind:checked={f.allowPrivate} aria-describedby="tf-priv" /> Allow a private address</label>
      <p class="help" id="tf-priv">Lets this server connect to this machine or your home network, such as 127.0.0.1 or 192.168.x.x. Leave it off for a server on the internet.</p>
    </div>
  {:else}
    <div class="field">
      <label for="tf-cmd">Command</label>
      <input id="tf-cmd" class="input mono" bind:value={f.command} autocomplete="off" spellcheck="false" autocapitalize="off" placeholder="npx" />
    </div>
    <div class="field">
      <label for="tf-args">Arguments (optional)</label>
      <textarea id="tf-args" class="input mono" rows="3" bind:value={argText} autocomplete="off" spellcheck="false" autocapitalize="off" aria-describedby="tf-args-help"></textarea>
      <p class="help" id="tf-args-help">One argument per line. Spaces inside a line stay in that one argument.</p>
    </div>
    <div class="field">
      <label for="tf-names">Environment variable names (optional)</label>
      <input id="tf-names" class="input mono" bind:value={names} autocomplete="off" spellcheck="false" autocapitalize="off" aria-describedby="tf-names-help" placeholder="API_KEY" />
      <p class="help" id="tf-names-help">Separate with spaces. You enter the values on the next screen, where they go to the secret store.</p>
    </div>
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="acts">
    <button type="button" class="btn" onclick={onBack}>Cancel</button>
    <button type="submit" class="btn primary">Review</button>
  </div>
</form>

<style>
  form {
    display: grid;
    gap: var(--s-3);
  }
  fieldset {
    display: grid;
    gap: 0;
    margin: 0;
    padding: 0;
    border: 0;
  }
  legend {
    padding: 0;
    font-size: var(--fs-s);
    font-weight: 600;
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
    accent-color: var(--accent);
  }
  textarea.input {
    padding: var(--s-1) var(--s-2);
    resize: vertical;
    overflow-wrap: anywhere;
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--s-2);
  }
</style>
