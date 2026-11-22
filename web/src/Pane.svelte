<script lang="ts">
  import { actions, paneCount } from './lib/actions'
  import { zoneAt } from './lib/dropzone'
  import { groups, type Group } from './lib/layout'
  import { tabLabels } from './lib/tablabels'
  import { groupTitle, membersOf } from './lib/chatgroups'
  import { app } from './lib/state.svelte'
  import TabStrip from './TabStrip.svelte'
  import PaneBody from './PaneBody.svelte'
  import { fileLabels } from './lib/paths'
  import { terms } from './lib/termstore.svelte'
  import { chats } from './lib/chatstore.svelte'
  import { ssh } from './lib/sshstore.svelte'
  import PaneActions from './PaneActions.svelte'
  import type { DropTarget } from './lib/layout-edit'

  let { group }: { group: Group } = $props()
  const fileTabs = $derived(group.tabs.filter((t) => t.kind === 'file' && t.path))
  const fileNames = $derived(new Map(fileLabels(fileTabs.map((t) => t.path!)).map((n, i) => [fileTabs[i].id, n])))
  const labels = $derived(
    tabLabels(group.tabs, (id, kind) => (kind === 'terminal' ? terms.ui[id]?.title : kind === 'chat' ? chats.list[id]?.title : kind === 'group' ? groupTitle(membersOf(chats.list, id)) : kind === 'file' ? fileNames.get(id) : undefined), (h) => ssh.host(h)?.name),
  )
  const label = (t: { id: string }) => labels.get(t.id) ?? 'Tab'

  const MIME = 'application/x-mooring-tab'
  const total = $derived(paneCount(app.layout))
  const number = $derived(app.layout ? groups(app.layout.root).indexOf(group) + 1 : 0)
  const focused = $derived(group.tabs.some((t) => t.id === app.layout?.focus))
  const active = $derived(group.tabs.find((t) => t.id === group.active))
  const anchor = $derived(group.active || group.tabs[0]?.id || '')

  let zone = $state<DropTarget | null>(null)
  let body: HTMLElement | undefined = $state()

  function isTabDrag(e: DragEvent) {
    return !!e.dataTransfer?.types.includes(MIME)
  }

  function bodyOver(e: DragEvent) {
    if (!isTabDrag(e) || !anchor) return
    e.preventDefault()
    zone = zoneAt(body!.getBoundingClientRect(), e.clientX, e.clientY)
  }

  function bodyDrop(e: DragEvent) {
    const id = e.dataTransfer?.getData(MIME)
    const where = zone
    zone = null
    if (!id || !where) return
    e.preventDefault()
    actions.drop(id, anchor, where)
  }
</script>

<section class="pane" class:focused aria-label={number ? `Pane ${number} of ${total}` : 'Pane'} onpointerdowncapture={() => anchor && !focused && actions.focusTab(anchor)}>
  <div class="bar">
    <TabStrip {group} {label} {anchor} />
    {#if focused || total <= 1}<PaneActions {number} {total} />{/if}
  </div>
  <div
    class="body"
    role="tabpanel"
    tabindex="-1"
    id={'panel-' + (group.tabs[0]?.id ?? 'empty')}
    bind:this={body}
    ondragover={bodyOver}
    ondragleave={() => (zone = null)}
    ondrop={bodyDrop}
  >
    <PaneBody {active} empty={group.tabs.length === 0} />
    {#if zone}<div class={['drop', zone]} aria-hidden="true"></div>{/if}
  </div>
</section>

<style>
  .pane {
    container: pane / inline-size;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto 1fr;
    min-width: 0;
    min-height: 0;
    height: 100%;
    background: var(--bg);
  }
  .bar {
    display: flex;
    align-items: stretch;
    min-height: var(--bar-h);
    background: var(--surface);
    border-top: 1px solid transparent;
    border-bottom: 1px solid var(--border);
  }
  .focused .bar {
    border-top-color: var(--accent);
  }
  .body {
    position: relative;
    min-height: 0;
    overflow: auto;
  }
  .body:has(.term),
  .body:has(:global([data-chat-view])),
  .body:has(:global([data-editor-view])) {
    overflow: hidden;
  }
  .drop {
    position: absolute;
    background: var(--selected);
    border: 2px solid var(--accent);
    pointer-events: none;
    inset: 0;
  }
  .drop.left {
    right: 50%;
  }
  .drop.right {
    left: 50%;
  }
  .drop.top {
    bottom: 50%;
  }
  .drop.bottom {
    top: 50%;
  }
</style>
