<script lang="ts">
  // 40px: the title, the state and the "…" menu. Model and Mode sit by the composer.
  import { app } from './lib/state.svelte'
  import type { ChatInfo } from './lib/chatstore.svelte'
  import { attentionLabel } from './lib/chatfmt'
  import { actions } from './lib/actions'
  import { git } from './lib/gitstore.svelte'
  import { onMount } from 'svelte'
  import ChatMenu from './ChatMenu.svelte'
  import { chatUsage } from './lib/chatusage'
  import { readsProject } from './lib/chatfolders'

  let { id, info }: { id: string; info: ChatInfo } = $props()

  const STATE: Record<string, string> = {
    starting: 'Starting',
    idle: 'Ready',
    running: 'Replying',
    needs_login: '',
    stopped: 'Stopped',
  }
  // A chat in its own worktree: the worktree list names its branch.
  const tree = $derived(git.worktrees.find((w) => w.chatId === id || (!!info.spec?.folder?.path && w.folder === info.spec.folder.path)))
  onMount(() => {
    if (git.project === info.projectId) void git.loadWorktrees()
  })
  const project = $derived(readsProject(info) ? app.projects.find((p) => p.id === info.projectId) : undefined)
  const usage = $derived(chatUsage(info))
  // A permission request shows in the banner by the composer, not here. Without a connection nothing here is current.
  const status = $derived(app.status !== 'live' ? 'Offline' : info.limitOnly ? attentionLabel(info) : (STATE[info.state] ?? info.state))
</script>

<header class="head">
  <div class="line">
    <h2 class="title">{info.title || 'New chat'}</h2>
    {#if usage}<span class="usage" title={`Plan usage of ${usage.name}\n${usage.all}`}>{usage.text}<span class="sr"> on {usage.name}</span></span>{/if}
    {#if status}<span class="state">{status}</span>{/if}
    <ChatMenu {id} {info} />
  </div>
  {#if tree}
    <div class="line tree">
      <span class="branch">Worktree <code>{tree.branch}</code></span>
      <button type="button" class="btn small" onclick={() => actions.showChanges(tree.folder)}>Open changes<span class="sr"> of {tree.branch}</span></button>
      {#if project}<span class="reads" title={project.path}>Can also read {project.name}</span>{/if}
    </div>
  {/if}
</header>

<style>
  .head {
    display: grid;
    padding: 0 max(var(--s-4), calc((100% - var(--chat-col)) / 2));
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .line {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    min-width: 0;
    min-height: 40px;
  }
  .title {
    flex: 1;
    min-width: 0;
    margin: 0;
    overflow: hidden;
    font-size: var(--fs-l);
    line-height: 20px;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .state {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .usage {
    flex: none;
    max-width: 50%;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* The header is one 40px row on a phone; the usage is in the "…" menu there. */
  @media (max-width: 700px) {
    .usage {
      display: none;
    }
  }
  .tree {
    min-height: 0;
    padding-bottom: var(--s-1);
    gap: var(--s-2);
    font-size: var(--fs-s);
  }
  .reads {
    min-width: 0;
    overflow-wrap: anywhere;
    color: var(--muted);
  }
  .branch {
    min-width: 0;
    overflow-wrap: anywhere;
    color: var(--muted);
  }
</style>
