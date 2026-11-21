<script lang="ts">
  // The consent screen. It shows exactly what will run, and is the only place that sends allow:true
  // and the only place secret values are typed. Values are held here until the save answers, then dropped.
  import { onDestroy, untrack } from 'svelte'
  import { CommandError } from './lib/events'
  import type { ServerDraft, ToolServer, Var } from './lib/toolapi'
  import { argsOf, explain, hasSecrets, isRemote, kindLabel, missingRequired, secretsPayload, type Entry } from './lib/toolfmt'
  import { tools } from './lib/toolstore.svelte'

  let { draft, missing = '', onBusy, onBack, onSaved }: { draft: ServerDraft; missing?: string; onBusy?: (b: boolean) => void; onBack: () => void; onSaved: (s: ToolServer) => void } = $props()

  const start = (vs: Var[]): Record<string, Entry> => Object.fromEntries(vs.map((v) => [v.name, { mode: v.set ? 'keep' : 'replace', value: '' } as Entry]))
  let envEntries = $state(untrack(() => start(draft.env)))
  let headerEntries = $state(untrack(() => start(draft.headers)))
  let error = $state('')
  let busy = $state(false)
  let gone = false
  onDestroy(() => (gone = true))
  const remote = $derived(isRemote(draft.kind))
  const vars = $derived(remote ? draft.headers : draft.env)
  const entries = $derived(remote ? headerEntries : envEntries)
  const noStore = $derived(!tools.canSaveSecrets && vars.length > 0)

  async function save() {
    error = missingRequired(draft.env, draft.headers, envEntries, headerEntries)
    if (error) return
    busy = true
    onBusy?.(true)
    try {
      const secrets = secretsPayload(draft.env, draft.headers, envEntries, headerEntries)
      const s = await tools.save(draft, hasSecrets(secrets) ? secrets : undefined, true)
      for (const e of [...Object.values(envEntries), ...Object.values(headerEntries)]) e.value = ''
      if (!gone) onSaved(s)
    } catch (e) {
      error = explain(e instanceof CommandError ? e.code : '', e instanceof Error ? e.message : 'The tool server could not be saved. Try again.')
    } finally {
      busy = false
      onBusy?.(false)
    }
  }
</script>

<form onsubmit={(e) => { e.preventDefault(); void save() }} aria-label="Allow {draft.name}">
  <p class="lead">
    {#if remote}
      Mooring and the agents you pick will connect to this address. Review it, then allow it.
    {:else}
      Mooring will run this command on this computer, as you, to check it and whenever a chat that uses it starts. Review it, then allow it. Mooring then checks it and enables it.
    {/if}
  </p>
  {#if missing}<p class="warn" role="note">{missing} is not installed on this computer. If you allow it now, the check fails until you install it, and it stays disabled.</p>{/if}
  <dl tabindex="-1" data-first aria-label="What will run">
    <dt>Name</dt>
    <dd>{draft.name}</dd>
    <dt>Type</dt>
    <dd>{kindLabel(draft)}{draft.source.registry ? `, from the catalog (${draft.source.registry} ${draft.source.version ?? ''})` : ', added by you'}</dd>
    {#if remote}
      <dt>Address</dt>
      <dd class="cmd" data-consent="address">{draft.url}</dd>
      {#if draft.allowPrivate}
        <dt>Private address</dt>
        <dd>Allowed. This server can connect to this machine or your home network.</dd>
      {/if}
    {:else}
      <dt>Command</dt>
      <dd class="cmd" data-consent="command">{argsOf(draft)[0]}</dd>
      <dt>Arguments</dt>
      <dd>
        {#if draft.args.length === 0}
          None.
        {:else}
          <ol>
            {#each draft.args as a, i (i)}<li class="cmd" data-consent="arg">{a}</li>{/each}
          </ol>
        {/if}
      </dd>
    {/if}
  </dl>

  {#if vars.length > 0}
    <h3>{remote ? 'Headers' : 'Environment variables'}</h3>
    {#if noStore}<p class="help">This computer has no secret store, so values cannot be saved. Leave the fields empty.</p>
    {:else}<p class="help">Values go to the secret store of the computer Mooring runs on. They are never shown again, and never kept in Mooring's database.</p>{/if}
    {#each vars as v (v.name)}
      {@const e = entries[v.name]}
      <div class="field var">
        <label for="tc-v-{v.name}"><code>{v.name}</code>{v.required ? ' (required)' : ''}</label>
        {#if v.description}<p class="help" id="tc-d-{v.name}">{v.description}</p>{/if}
        {#if v.set && e.mode === 'keep'}
          <div class="saved">
            <span id="tc-v-{v.name}">Saved</span>
            <button type="button" class="btn" aria-label="Replace {v.name}" onclick={() => (e.mode = 'replace')}>Replace</button>
            <button type="button" class="btn" aria-label="Clear {v.name}" onclick={() => (e.mode = 'clear')}>Clear</button>
          </div>
        {:else if e.mode === 'clear'}
          <div class="saved">
            <span id="tc-v-{v.name}">The saved value will be deleted.</span>
            <button type="button" class="btn" aria-label="Keep {v.name}" onclick={() => (e.mode = 'keep')}>Keep it</button>
          </div>
        {:else}
          <input id="tc-v-{v.name}" class="input mono" type="password" bind:value={e.value} autocomplete="new-password" spellcheck="false" autocapitalize="off" aria-describedby={v.description ? `tc-d-${v.name}` : undefined} />
          {#if v.set}<button type="button" class="btn" aria-label="Keep the saved {v.name}" onclick={() => ((e.mode = 'keep'), (e.value = ''))}>Keep the saved value</button>{/if}
        {/if}
      </div>
    {/each}
  {/if}

  {#if remote}<p class="help">If the server asks you to sign in, you do that after the check.</p>{/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="acts">
    <button type="button" class="btn" disabled={busy} onclick={onBack}>Back</button>
    <button type="submit" class="btn primary" disabled={busy}>{busy ? 'Allowing…' : 'Allow and enable'}</button>
  </div>
</form>

<style>
  form {
    display: grid;
    gap: var(--s-3);
  }
  p,
  h3 {
    margin: 0;
  }
  h3 {
    font-size: var(--fs-m);
  }
  dl {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: var(--s-1) var(--s-3);
    margin: 0;
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
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .cmd {
    font-family: var(--font-mono);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  ol {
    display: grid;
    gap: var(--s-1);
    margin: 0;
    padding-left: var(--s-5);
  }
  .warn {
    color: var(--attention);
    font-size: var(--fs-s);
  }
  .var {
    gap: var(--s-1);
  }
  .saved {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
  }
  .saved span {
    flex: 1 1 8em;
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--s-2);
  }
</style>
