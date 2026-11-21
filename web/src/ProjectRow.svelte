<script lang="ts">
  import { composing } from './lib/keys'
  import { tick } from 'svelte'
  import { floatMenu } from './lib/floatmenu'
  import { app, type Project } from './lib/state.svelte'
  import AttentionMark from './AttentionMark.svelte'
  import { chats } from './lib/chatstore.svelte'
  import { projectAttentionLabel } from './lib/chatfmt'

  let {
    project,
    index,
    onPick,
    onError,
  }: { project: Project; index: number; onPick: () => void; onError: (message: string) => void } = $props()

  const MIME = 'application/x-mooring-project'
  const current = $derived(project.id === app.currentId)
  const last = $derived(index === app.projects.length - 1)
  const waiting = $derived(chats.attention(project.id))

  let menu = $state<'closed' | 'open' | 'confirm'>('closed')
  let renaming = $state(false)
  let draft = $state('')
  let over = $state<'before' | 'after' | null>(null)
  let root: HTMLElement | undefined = $state()
  let trigger: HTMLButtonElement | undefined = $state()
  let menuEl: HTMLElement | undefined = $state()

  async function openMenu() {
    menu = 'open'
    await tick()
    menuEl?.querySelector<HTMLElement>('button')?.focus({ preventScroll: true })
  }

  function closeMenu(refocus = true) {
    menu = 'closed'
    if (refocus) trigger?.focus()
  }

  function menuKeys(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation()
      return closeMenu()
    }
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0
    if (!step) return
    e.preventDefault()
    const items = [...menuEl!.querySelectorAll<HTMLElement>('button')]
    const i = items.indexOf(document.activeElement as HTMLElement)
    items[(i + step + items.length) % items.length]?.focus()
  }

  async function startRename() {
    draft = project.name
    renaming = true
    closeMenu(false)
    await tick()
    const input = root?.querySelector('input')
    input?.focus()
    input?.select()
  }

  async function commitRename() {
    if (!renaming) return
    renaming = false
    const name = draft.trim()
    if (!name || name === project.name) return
    try {
      await app.rename(project.id, name)
    } catch (e) {
      onError(e instanceof Error ? e.message : 'Could not rename the project.')
    }
  }

  async function move(to: number) {
    closeMenu()
    try {
      await app.move(project.id, to)
    } catch (e) {
      onError(e instanceof Error ? e.message : 'Could not reorder the projects.')
    }
  }

  async function remove() {
    closeMenu(false)
    try {
      await app.remove(project.id)
    } catch (e) {
      onError(e instanceof Error ? e.message : 'Could not remove the project.')
    }
  }

  function dragOver(e: DragEvent) {
    if (!e.dataTransfer?.types.includes(MIME)) return
    e.preventDefault()
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    over = e.clientY < r.top + r.height / 2 ? 'before' : 'after'
  }

  function drop(e: DragEvent) {
    const id = e.dataTransfer?.getData(MIME)
    const side = over
    over = null
    if (!id || !side) return
    e.preventDefault()
    const from = app.projects.findIndex((p) => p.id === id)
    let to = index + (side === 'after' ? 1 : 0)
    if (from < to) to--
    void app.move(id, to).catch((err: unknown) => onError(err instanceof Error ? err.message : 'Could not reorder the projects.'))
  }
</script>

<svelte:window onpointerdown={(e) => menu !== 'closed' && !root?.contains(e.target as Node) && closeMenu(false)} />

<li
  class="row"
  class:current
  class:before={over === 'before'}
  class:after={over === 'after'}
  bind:this={root}
  draggable={!renaming}
  ondragstart={(e) => e.dataTransfer?.setData(MIME, project.id)}
  ondragover={dragOver}
  ondragleave={() => (over = null)}
  ondrop={drop}
>
  {#if renaming}
    <input
      class="input"
      aria-label={`New name for ${project.name}`}
      bind:value={draft}
      onblur={commitRename}
      onkeydown={(e) => {
        if (e.key === 'Enter' && !composing(e)) commitRename()
        if (e.key === 'Escape') renaming = false
      }}
    />
  {:else}
    <button type="button" class="name" aria-current={current ? 'true' : undefined} title={project.path} onclick={onPick}>{project.name}</button>
    {#if waiting}<AttentionMark label={projectAttentionLabel(Object.values(chats.list).filter((c) => c.projectId === project.id))} />{/if}
    <button
      type="button"
      class="more"
      bind:this={trigger}
      aria-label={`Actions for ${project.name}`}
      aria-haspopup="menu"
      aria-expanded={menu !== 'closed'}
      onclick={() => (menu === 'closed' ? openMenu() : closeMenu())}>…</button>
  {/if}
  {#if menu !== 'closed'}
    <div class="menu" role={menu === 'open' ? 'menu' : 'group'} aria-label={`Actions for ${project.name}`} tabindex="-1" bind:this={menuEl} onkeydown={menuKeys} use:floatMenu={{ anchor: () => trigger, close: () => closeMenu() }}>
      {#if menu === 'open'}
        <button type="button" role="menuitem" onclick={startRename}>Rename</button>
        <button type="button" role="menuitem" disabled={index === 0} onclick={() => move(index - 1)}>Move up</button>
        <button type="button" role="menuitem" disabled={last} onclick={() => move(index + 1)}>Move down</button>
        <button type="button" role="menuitem" class="danger" onclick={() => (menu = 'confirm')}>Remove from list…</button>
      {:else}
        <p>Remove “{project.name}” from the list? Its files stay on disk in <code>{project.path}</code>.</p>
        <div class="confirm">
          <button type="button" class="btn danger" onclick={remove}>Remove from list</button>
          <button type="button" class="btn" onclick={() => closeMenu()}>Cancel</button>
        </div>
      {/if}
    </div>
  {/if}
</li>

<style>
  .row {
    position: relative;
    display: flex;
    align-items: center;
    min-height: var(--control-h);
    border-radius: var(--r-s);
    list-style: none;
    transition: background-color var(--ease);
  }
  .row:hover {
    background: var(--hover);
  }
  .row.current {
    background: var(--selected);
    box-shadow: inset 2px 0 var(--accent);
  }
  .row.before::before,
  .row.after::after {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    height: 2px;
    background: var(--accent);
  }
  .row.before::before {
    top: -1px;
  }
  .row.after::after {
    bottom: -1px;
  }
  .name {
    flex: 1;
    min-width: 0;
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 0;
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .current .name {
    font-weight: 600;
  }
  .more {
    flex: none;
    width: var(--control-h);
    min-height: var(--control-h);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--muted);
    font: inherit;
    opacity: 0;
  }
  .row:hover .more,
  .row:focus-within .more,
  .more[aria-expanded='true'] {
    opacity: 1;
  }
  .more:hover {
    color: var(--fg);
  }
  .menu {
    display: grid;
    min-width: 200px;
    max-width: 260px;
    padding: var(--s-1);
    border: 1px solid var(--border);
    border-radius: var(--r-m);
    background: var(--raised);
    box-shadow: var(--shadow-menu);
    color: var(--fg);
  }
  .menu > button {
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .menu > button:hover:not(:disabled) {
    background: var(--hover);
  }
  .menu > button:disabled {
    color: var(--muted);
  }
  .menu > .danger {
    color: var(--danger);
  }
  .menu p {
    margin: 0;
    padding: var(--s-2);
    color: var(--fg);
    overflow-wrap: anywhere;
  }
  .confirm {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
    padding: var(--s-2);
  }
  .input {
    margin: 2px var(--s-1);
    width: calc(100% - var(--s-2));
  }
  @media (pointer: coarse) {
    .more {
      opacity: 1;
    }
  }
</style>
