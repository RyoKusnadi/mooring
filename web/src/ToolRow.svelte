<script lang="ts">
  // One saved tool server: what it is, its state, its tools, and what can be done with it.
  import { tick } from 'svelte'
  import type { ToolServer } from './lib/toolapi'
  import { argsOf, explain, isRemote, kindLabel, statusOf } from './lib/toolfmt'
  import { tools } from './lib/toolstore.svelte'

  let { server: s, onEdit, onCheck, onSignIn, onReview, onGone }: { server: ToolServer; onEdit: () => void; onCheck: () => void; onSignIn: () => void; onReview: () => void; onGone: () => void } = $props()

  let removing = $state(false)
  let open = $state(false)
  let busy = $state(false)
  let error = $state('')
  let removeBtn: HTMLButtonElement | undefined = $state()
  let keepBtn: HTMLButtonElement | undefined = $state()

  const states = $derived(statusOf(s, tools.failures[s.id]))
  const remote = $derived(isRemote(s.kind))
  const line = $derived(remote ? s.url : argsOf(s).join(' '))

  async function act(f: () => Promise<unknown>, fallback: string) {
    busy = true
    error = ''
    try {
      await f()
    } catch (e) {
      error = explain((e as { code?: string }).code ?? '', e instanceof Error ? e.message : fallback)
    } finally {
      busy = false
    }
  }

  const toggle = () => act(() => tools.enable(s.id, !s.enabled), 'Could not change this tool server. Try again.')
  const signOut = () => act(() => tools.signOut(s.id), 'Could not sign out. Try again.')
  async function remove() {
    await act(() => tools.remove(s.id), 'The tool server could not be removed. Try again.')
    if (!error) onGone()
  }
  async function ask(on: boolean) {
    removing = on
    await tick()
    ;(on ? keepBtn : removeBtn)?.focus()
  }
</script>

<li>
  <div class="what">
    <strong>{s.name}</strong>
    <span class="muted">{kindLabel(s)}{s.source.registry ? `, from the catalog, version ${s.source.version ?? ''}` : ', added by you'}</span>
    <code>{line}</code>
    <span class="states">
      {#each states as st (st.text)}<span class="state" class:att={st.attention}>{#if st.attention}<span class="dot" aria-hidden="true"></span>{/if}{st.text}</span>{/each}
    </span>
    {#if tools.failures[s.id] && !s.enabled}<p class="fail">{tools.failures[s.id]}</p>{/if}
    {#if s.tools.length > 0}
      <button type="button" class="link" aria-expanded={open} aria-controls="tl-{s.id}" onclick={() => (open = !open)}>{open ? 'Hide' : 'Show'} {s.tools.length} {s.tools.length === 1 ? 'tool' : 'tools'}<span class="sr"> of {s.name}</span></button>
      {#if open}
        <ul id="tl-{s.id}" class="tl">
          {#each s.tools as t (t.name)}<li><code>{t.name}</code>{#if t.description}<span class="muted tdesc">{t.description}</span>{/if}</li>{/each}
        </ul>
      {/if}
    {/if}
  </div>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if removing}
    <div class="confirm" role="group" aria-label="Remove {s.name}">
      <span>Remove {s.name}? Its saved values and sign-in are deleted. Chats already running keep it until they stop.</span>
      <button type="button" class="btn danger solid" disabled={busy} onclick={remove}>Remove</button>
      <button type="button" class="btn" bind:this={keepBtn} data-keep onclick={() => ask(false)}>Keep</button>
    </div>
  {:else}
    <div class="acts">
      {#if !s.allowed}
        <button type="button" class="btn primary" aria-label="Review and allow {s.name}" onclick={onReview}>Review and allow</button>
      {:else}
        <button type="button" class="btn" class:primary={!s.checkedAt && !s.enabled} aria-label="Check {s.name}" disabled={busy} onclick={onCheck}>Check</button>
        <button type="button" class="btn" aria-label="{s.enabled ? 'Disable' : 'Enable'} {s.name}" disabled={busy} onclick={toggle}>{s.enabled ? 'Disable' : 'Enable'}</button>
        {#if remote && s.signIn !== 'signedIn'}
          <button type="button" class="btn" aria-label="Sign in to {s.name}" onclick={onSignIn}>Sign in</button>
        {:else if remote}
          <button type="button" class="btn" aria-label="Sign out of {s.name}" disabled={busy} onclick={signOut}>Sign out</button>
        {/if}
      {/if}
      <button type="button" class="btn" aria-label="Edit {s.name}" onclick={onEdit}>Edit</button>
      <button type="button" class="btn danger" aria-label="Remove {s.name}" bind:this={removeBtn} onclick={() => ask(true)}>Remove</button>
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
  code {
    max-width: 100%;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }
  .muted {
    color: var(--muted);
    overflow-wrap: anywhere;
  }
  .states {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-3);
    margin-top: var(--s-1);
  }
  .state {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    font-weight: var(--fw-semibold);
  }
  .state.att {
    padding: 0 var(--s-1);
    border-radius: var(--r-s);
    background: var(--attention-bg);
    color: var(--attention);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--attention);
  }
  .fail {
    margin: var(--s-1) 0 0;
    max-width: 100%;
    color: var(--danger);
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }
  .link {
    min-height: var(--control-h);
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
  }
  .tl {
    display: grid;
    gap: var(--s-1);
    margin: 0;
    padding: 0 0 0 var(--s-4);
    border: 0;
    list-style: disc;
  }
  .tdesc {
    display: block;
  }
  .tl li {
    display: list-item;
    padding: 0;
    border: 0;
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
