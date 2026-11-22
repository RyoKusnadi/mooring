<script lang="ts">
  import ChatLazy from './ChatLazy.svelte'
  import ChatGroupLazy from './ChatGroupLazy.svelte'
  import ChangesLazy from './ChangesLazy.svelte'
  import DiffLazy from './DiffLazy.svelte'
  import EmptyPane from './EmptyPane.svelte'
  import FileLazy from './FileLazy.svelte'
  import HistoryLazy from './HistoryLazy.svelte'
  import RemoteTab from './RemoteTab.svelte'
  import Terminal from './Terminal.svelte'
  import type { Tab } from './lib/layout'

  let { active, empty }: { active: Tab | undefined; empty: boolean } = $props()
</script>

{#if empty}
  <EmptyPane />
{:else if active?.kind === 'terminal'}
  {#key active.id}<Terminal id={active.id} />{/key}
{:else if active?.kind === 'chat'}
  {#key active.id}<ChatLazy id={active.id} />{/key}
{:else if active?.kind === 'group'}
  {#key active.id}<ChatGroupLazy id={active.id} />{/key}
{:else if active?.kind === 'file' && active.path !== undefined}
  <!-- The path is in the key: a renamed file gets a fresh view that keeps the tab's state. -->
  {#key active.id + ':' + active.path}<FileLazy id={active.id} path={active.path} />{/key}
{:else if active?.kind === 'diff' && active.path !== undefined}
  {#key active.id + ':' + !!active.staged}<DiffLazy id={active.id} folder={active.folder ?? ''} path={active.path} staged={!!active.staged} commit={active.commit ?? ''} />{/key}
{:else if active?.kind === 'changes'}
  {#key active.id}<ChangesLazy />{/key}
{:else if active?.kind === 'history'}
  {#key active.id}<HistoryLazy folder={active.folder ?? ''} />{/key}
{:else if active?.kind === 'commit' && active.commit}
  {#key active.id}<HistoryLazy folder={active.folder ?? ''} commit={active.commit} />{/key}
{:else if (active?.kind === 'remote' || active?.kind === 'rfile') && active.host}
  {#key active.id + ':' + active.path}<RemoteTab tab={active} />{/key}
{:else}
  <EmptyPane id={active?.id ?? ''} />
{/if}
