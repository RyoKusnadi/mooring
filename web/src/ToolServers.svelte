<script lang="ts">
  // The "Tool servers" dialog. One view at a time: the list, the catalog, the add-your-own form,
  // the consent screen, the health check and the sign-in. Nothing runs before the consent screen.
  import { onMount, tick } from 'svelte'
  import SshModal from './SshModal.svelte'
  import ToolCatalog from './ToolCatalog.svelte'
  import ToolCheck from './ToolCheck.svelte'
  import ToolConsent from './ToolConsent.svelte'
  import ToolForm from './ToolForm.svelte'
  import ToolRow from './ToolRow.svelte'
  import ToolSignIn from './ToolSignIn.svelte'
  import type { ServerDraft, ToolServer } from './lib/toolapi'
  import { draftOf, emptyDraft } from './lib/toolfmt'
  import { tools } from './lib/toolstore.svelte'

  let { onClose }: { onClose: () => void } = $props()

  type View =
    | { v: 'list' }
    | { v: 'catalog' }
    | { v: 'form'; draft: ServerDraft }
    | { v: 'consent'; draft: ServerDraft; back: 'catalog' | 'form' | 'list'; missing?: string }
    | { v: 'check'; id: string; auto?: boolean }
    | { v: 'signin'; id: string }

  let view = $state<View>({ v: 'list' })
  let error = $state('')
  let box: HTMLElement | undefined = $state()
  let seen = ''
  let consentBusy = $state(false)

  // A new view takes the focus to its own first control.
  $effect(() => {
    const v = view.v
    if (seen && v !== seen) void tick().then(() => box?.querySelector<HTMLElement>('[data-first]')?.focus())
    seen = v
  })

  onMount(() => {
    void tools.load().catch(() => (error = 'Could not read the saved tool servers. Check that Mooring is running.'))
  })

  const list = () => (view = { v: 'list' })
  const named = (id: string) => tools.get(id)?.name ?? 'tool server'
  const title = $derived(
    view.v === 'catalog'
      ? 'Find a tool server'
      : view.v === 'form'
        ? view.draft.id
          ? 'Edit tool server'
          : 'Add your own tool server'
        : view.v === 'consent'
          ? 'Allow this tool server?'
          : view.v === 'check'
            ? `Check ${named(view.id)}`
            : view.v === 'signin'
              ? `Sign in to ${named(view.id)}`
              : 'Tool servers',
  )

  // Escape closes the inner step first: a remove confirm, then any view but the list.
  function onCancel(): boolean {
    const keep = box?.querySelector<HTMLButtonElement>('[data-keep]')
    if (keep) return keep.click(), true
    if (view.v === 'list') return false
    if (view.v === 'consent' && consentBusy) return true
    view = view.v === 'consent' && view.back === 'form' ? { v: 'form', draft: view.draft } : view.v === 'consent' && view.back === 'catalog' ? { v: 'catalog' } : { v: 'list' }
    return true
  }

  function edit(s: ToolServer) {
    // A catalog server keeps the command Mooring built from its package, so only its values change.
    view = s.source.registry ? { v: 'consent', draft: draftOf(s), back: 'list' } : { v: 'form', draft: draftOf(s) }
  }
</script>

<SshModal {title} {onClose} wide top {onCancel}>
  <div bind:this={box}>
    {#if view.v === 'catalog'}
      <ToolCatalog onBack={list} onPick={(draft, missing) => (view = { v: 'consent', draft, back: 'catalog', missing })} />
    {:else if view.v === 'form'}
      {#key view.draft.id ?? 'new'}<ToolForm draft={view.draft} onBack={list} onNext={(draft) => (view = { v: 'consent', draft, back: 'form' })} />{/key}
    {:else if view.v === 'consent'}
      {@const c = view}
      <ToolConsent
        draft={c.draft}
        missing={c.missing}
        onBusy={(b) => (consentBusy = b)}
        onBack={() => (view = c.back === 'form' ? { v: 'form', draft: c.draft } : { v: c.back })}
        onSaved={(s) => (view = { v: 'check', id: s.id, auto: true })}
      />
    {:else if view.v === 'check'}
      {#key view.id}<ToolCheck id={view.id} auto={view.auto} onDone={list} onSignIn={(id) => (view = { v: 'signin', id })} />{/key}
    {:else if view.v === 'signin'}
      {#key view.id}<ToolSignIn id={view.id} onDone={list} />{/key}
    {:else}
      <div class="top">
        <button type="button" class="btn primary" data-first onclick={() => (view = { v: 'catalog' })}>Find a tool server</button>
        <button type="button" class="btn" onclick={() => (view = { v: 'form', draft: emptyDraft() })}>Add your own</button>
      </div>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      {#if !tools.loaded && !error}
        <p class="muted" role="status">Loading…</p>
      {:else if tools.servers.length === 0}
        <p class="muted">No tool servers yet. A tool server gives an agent extra tools. Find one in the catalog, or add your own. Nothing runs until you allow it.</p>
      {:else}
        <ul>
          {#each tools.servers as s (s.id)}
            <ToolRow
              server={s}
              onEdit={() => edit(s)}
              onCheck={() => (view = { v: 'check', id: s.id })}
              onSignIn={() => (view = { v: 'signin', id: s.id })}
              onReview={() => (view = { v: 'consent', draft: draftOf(s), back: 'list' })}
              onGone={() => void tick().then(() => box?.querySelector<HTMLElement>('[data-first]')?.focus())}
            />
          {/each}
        </ul>
      {/if}
    {/if}
  </div>
</SshModal>

<style>
  .top {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
    margin-bottom: var(--s-3);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border);
  }
  .muted {
    margin: 0;
    color: var(--muted);
  }
</style>
